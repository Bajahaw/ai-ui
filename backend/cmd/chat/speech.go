package chat

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Bajahaw/ai-ui/cmd/auth"
	"github.com/Bajahaw/ai-ui/cmd/data"
	"github.com/Bajahaw/ai-ui/cmd/providers"
	"github.com/Bajahaw/ai-ui/cmd/utils"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// maxTTSInputChars is the OpenAI /audio/speech input limit.
const maxTTSInputChars = 4096

// TTSHandler serves GET /api/tts/messages/{id} for assistant message read-aloud.
// Audio is browser-cacheable via ETag + Cache-Control (private).
func TTSHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /messages/{id}", synthesizeMessageSpeech)
	return http.StripPrefix("/api/tts", auth.Authenticated(mux))
}

func synthesizeMessageSpeech(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)

	idStr := r.PathValue("id")
	messageID, err := strconv.Atoi(idStr)
	if err != nil || messageID <= 0 {
		http.Error(w, "Invalid message id", http.StatusBadRequest)
		return
	}

	msg, err := getMessage(messageID, user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Message not found", http.StatusNotFound)
			return
		}
		log.Error("TTS failed loading message", "id", messageID, "err", err)
		http.Error(w, "Failed to load message", http.StatusInternalServerError)
		return
	}

	if msg.Role != "assistant" {
		http.Error(w, "Read aloud is only available for assistant messages", http.StatusBadRequest)
		return
	}
	if msg.Status == "pending" {
		http.Error(w, "Message is still generating", http.StatusConflict)
		return
	}
	if strings.TrimSpace(msg.Error) != "" {
		http.Error(w, "Cannot read aloud a failed message", http.StatusBadRequest)
		return
	}

	text := strings.TrimSpace(stripMarkdownForTTS(msg.Content))
	if text == "" {
		http.Error(w, "Message has no readable text", http.StatusBadRequest)
		return
	}
	if utf8.RuneCountInString(text) > maxTTSInputChars {
		text = truncateRunes(text, maxTTSInputChars)
	}

	ttsModel, err := settings.Get("ttsModel", user)
	if err != nil || strings.TrimSpace(ttsModel) == "" {
		http.Error(w, "No TTS model configured. Select a TTS model in Settings → Media.", http.StatusBadRequest)
		return
	}

	voice, _ := settings.Get("ttsVoice", user)
	voice = strings.TrimSpace(voice)
	if voice == "" {
		voice = "alloy"
	}

	speed := 1.0
	if speedStr, err := settings.Get("ttsSpeed", user); err == nil {
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(speedStr), 64); err == nil {
			speed = parsed
		}
	}
	speed = clampTTSSpeed(speed)

	// ETag covers content + TTS settings so edits / setting changes invalidate cache.
	etag := ttsETag(messageID, text, ttsModel, voice, speed)

	// must-revalidate: browser may store the MP3 but rechecks ETag so a voice/speed
	// change or message edit does not keep serving stale audio. Matching ETag → 304
	// (no OpenAI call, body served from browser cache).
	// Override global /api/ no-store middleware so caching can work.
	setTTSCacheHeaders := func() {
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
		w.Header().Set("Vary", "Cookie")
		w.Header().Del("Pragma")
		w.Header().Del("Expires")
	}

	if match := strings.TrimSpace(r.Header.Get("If-None-Match")); match != "" {
		if etagMatches(match, etag) {
			setTTSCacheHeaders()
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	providerID, modelName := utils.ExtractProviderID(ttsModel)
	if providerID == "" || modelName == "" {
		http.Error(w, "Invalid TTS model. Select a model from a provider in Settings → Media.", http.StatusBadRequest)
		return
	}

	providerRepo := providers.NewRepository(data.DB)
	provider, err := providerRepo.GetByID(providerID, user)
	if err != nil {
		log.Error("TTS provider not found", "provider", providerID, "err", err)
		http.Error(w, "TTS provider not found. Check your TTS model setting.", http.StatusBadRequest)
		return
	}

	opts := []option.RequestOption{
		option.WithAPIKey(provider.APIKey),
		option.WithBaseURL(provider.BaseURL),
	}
	for key, value := range provider.Headers {
		opts = append(opts, option.WithHeader(key, value))
	}
	client := openai.NewClient(opts...)

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	resp, err := client.Audio.Speech.New(ctx, openai.AudioSpeechNewParams{
		Input: text,
		Model: modelName,
		Voice: openai.AudioSpeechNewParamsVoiceUnion{
			OfString: openai.String(voice),
		},
		Speed:          openai.Float(speed),
		ResponseFormat: openai.AudioSpeechNewParamsResponseFormatMP3,
	})
	if err != nil {
		log.Error("TTS speech request failed", "model", modelName, "err", err)
		http.Error(w, "TTS request failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "audio/mpeg"
	}
	setTTSCacheHeaders()
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Error("TTS failed writing audio stream", "err", err)
	}
}

func ttsETag(messageID int, text, model, voice string, speed float64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf(
		"%d\n%s\n%s\n%s\n%g",
		messageID, text, model, voice, speed,
	)))
	return `"` + hex.EncodeToString(sum[:16]) + `"`
}

func etagMatches(ifNoneMatch, etag string) bool {
	// If-None-Match may be a comma-separated list or *.
	if ifNoneMatch == "*" {
		return true
	}
	for _, part := range strings.Split(ifNoneMatch, ",") {
		candidate := strings.TrimSpace(part)
		// Weak validators: W/"..."
		candidate = strings.TrimPrefix(candidate, "W/")
		if candidate == etag {
			return true
		}
	}
	return false
}

func clampTTSSpeed(speed float64) float64 {
	if speed < 0.25 {
		return 0.25
	}
	if speed > 4.0 {
		return 4.0
	}
	return speed
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max])
}

// stripMarkdownForTTS removes common markdown so the model does not speak
// syntax characters. Keeps link labels and code content as plain text.
func stripMarkdownForTTS(s string) string {
	s = fencedCodeRe.ReplaceAllString(s, "$1")
	s = inlineCodeRe.ReplaceAllString(s, "$1")
	s = imageMdRe.ReplaceAllString(s, "$1")
	s = linkMdRe.ReplaceAllString(s, "$1")
	s = headingMdRe.ReplaceAllString(s, "")
	s = boldMdRe.ReplaceAllString(s, "$1$2")
	s = italicMdRe.ReplaceAllString(s, "$1$2")
	s = strikeMdRe.ReplaceAllString(s, "$1")
	s = quoteListMdRe.ReplaceAllString(s, "")
	s = hrMdRe.ReplaceAllString(s, " ")
	s = whitespaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
