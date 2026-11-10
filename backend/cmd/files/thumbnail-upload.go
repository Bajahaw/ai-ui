package files

import (
	"bytes"
	"image"
	"io"
	"net/http"
	"strings"

	"github.com/Bajahaw/ai-ui/cmd/utils"
)

const maxThumbnailUploadBytes = 2 << 20

func isOfficeMIME(mimeType string) bool {
	switch strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0])) {
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return true
	default:
		return false
	}
}

// uploadThumbnail stores a derived image, never modifying the original file.
// First writer wins, matching the immutable thumbnail URL/cache policy.
func uploadThumbnail(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	owned, err := repo.GetByIDs([]string{r.PathValue("id")}, user)
	if err != nil || len(owned) != 1 {
		http.NotFound(w, r)
		return
	}
	file := owned[0]
	if !isOfficeMIME(file.Type) && !isThumbnailableMIME(file.Type) {
		http.Error(w, "unsupported thumbnail source", http.StatusUnsupportedMediaType)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxThumbnailUploadBytes)
	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "thumbnail exceeds 2MB limit", http.StatusRequestEntityTooLarge)
		return
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png") {
		http.Error(w, "thumbnail must be a valid PNG or JPEG", http.StatusUnsupportedMediaType)
		return
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 4096 || cfg.Height > 4096 || int64(cfg.Width)*int64(cfg.Height) > 4<<20 {
		http.Error(w, "thumbnail dimensions exceed limit", http.StatusBadRequest)
		return
	}
	if err := WriteThumbnailFromBytes(data, file.Path); err != nil {
		http.Error(w, "could not store thumbnail", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
