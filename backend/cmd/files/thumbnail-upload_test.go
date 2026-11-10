package files

import (
	"bytes"
	"context"
	"image"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"
)

func seedThumbnailOffice(t *testing.T, extension, mimeType string) File {
	t.Helper()
	file := File{ID: "office-" + extension, Name: "Report." + extension, Type: mimeType, Path: "data/resources/office-" + extension + "." + extension, User: "testuser"}
	if err := os.MkdirAll(path.Dir(file.Path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file.Path, []byte("original document bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(file); err != nil {
		t.Fatal(err)
	}
	return file
}

func postThumbnail(id, user string, data []byte) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /{id}/thumbnail", uploadThumbnail)
	req := httptest.NewRequest(http.MethodPost, "/"+id+"/thumbnail", bytes.NewReader(data))
	req = req.WithContext(context.WithValue(req.Context(), "user", user))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func fetchOfficeThumbnail(file File, user string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/thumbs/"+resourceStem(file.Path)+".jpg", nil)
	req = req.WithContext(context.WithValue(req.Context(), "user", user))
	rr := httptest.NewRecorder()
	ResourcesHandler(http.NotFoundHandler()).ServeHTTP(rr, req)
	return rr
}

func TestOfficeThumbnailUploadAndServe(t *testing.T) {
	setupUploadEnv(t)
	for _, format := range []struct{ extension, mimeType string }{
		{"docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{"pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation"},
	} {
		t.Run(format.extension, func(t *testing.T) {
			file := seedThumbnailOffice(t, format.extension, format.mimeType)
			if rr := fetchOfficeThumbnail(file, "testuser"); rr.Code != http.StatusNotFound {
				t.Fatalf("missing Office thumbnail must not render server-side: %d", rr.Code)
			}
			if rr := postThumbnail(file.ID, "other", pngBytes(t, 40, 30)); rr.Code != http.StatusNotFound {
				t.Fatalf("other user upload: %d", rr.Code)
			}
			if rr := postThumbnail(file.ID, "testuser", pngBytes(t, 800, 600)); rr.Code != http.StatusNoContent {
				t.Fatalf("upload: %d %s", rr.Code, rr.Body.String())
			}
			rr := fetchOfficeThumbnail(file, "testuser")
			if rr.Code != http.StatusOK {
				t.Fatalf("serve: %d %s", rr.Code, rr.Body.String())
			}
			cfg, format, err := image.DecodeConfig(bytes.NewReader(rr.Body.Bytes()))
			if err != nil || format != "jpeg" || cfg.Width != 360 || cfg.Height != 270 {
				t.Fatalf("stored thumbnail: %v %s %v", cfg, format, err)
			}
			if rr.Header().Get("Cache-Control") != "private, max-age=31536000, immutable" {
				t.Fatal("thumbnail must be privately cached")
			}
			if rr := fetchOfficeThumbnail(file, "other"); rr.Code != http.StatusNotFound {
				t.Fatalf("other user read: %d", rr.Code)
			}
			before := append([]byte(nil), rr.Body.Bytes()...)
			if rr := postThumbnail(file.ID, "testuser", pngBytes(t, 10, 10)); rr.Code != http.StatusNoContent {
				t.Fatalf("repeat upload: %d", rr.Code)
			}
			if !bytes.Equal(before, fetchOfficeThumbnail(file, "testuser").Body.Bytes()) {
				t.Fatal("an existing immutable thumbnail must not be overwritten")
			}
			original, err := os.ReadFile(file.Path)
			if err != nil || string(original) != "original document bytes" {
				t.Fatal("original file was modified")
			}
		})
	}
}

func TestThumbnailUploadRejectsInvalidInputs(t *testing.T) {
	setupUploadEnv(t)
	file := seedThumbnailOffice(t, "docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	for _, tc := range []struct {
		name   string
		data   []byte
		status int
	}{
		{"empty", nil, http.StatusUnsupportedMediaType},
		{"svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), http.StatusUnsupportedMediaType},
		{"invalid", []byte("not an image"), http.StatusUnsupportedMediaType},
		{"oversized body", make([]byte, maxThumbnailUploadBytes+1), http.StatusRequestEntityTooLarge},
		{"oversized dimensions", pngBytes(t, 4097, 1), http.StatusBadRequest},
		{"oversized pixels", pngBytes(t, 2049, 2048), http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rr := postThumbnail(file.ID, "testuser", tc.data)
			if rr.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", rr.Code, tc.status, rr.Body.String())
			}
			if thumbExists(ThumbnailPath(file.Path)) {
				t.Fatal("invalid request created a thumbnail")
			}
		})
	}
	if rr := postThumbnail("missing", "testuser", pngBytes(t, 10, 10)); rr.Code != http.StatusNotFound {
		t.Fatalf("missing file: %d", rr.Code)
	}
	text := seedThumbnailOffice(t, "txt", "text/plain")
	if rr := postThumbnail(text.ID, "testuser", pngBytes(t, 10, 10)); rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("unsupported source: %d", rr.Code)
	}
}
