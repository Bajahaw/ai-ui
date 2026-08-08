package files

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func writeTestPNG(t *testing.T, p string, w, h int, c color.Color) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 40, B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}

func TestThumbnailPathConvention(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"data/resources/abc-123.png", "data/resources/thumbs/abc-123.jpg"},
		{"./data/resources/abc-123.JPEG", "data/resources/thumbs/abc-123.jpg"},
		{"data/resources/abc-123", "data/resources/thumbs/abc-123.jpg"},
		{"", ""},
		{"data/resources/../evil.png", ""},
		{`data\resources\abc-123.png`, "data/resources/thumbs/abc-123.jpg"},
	}
	for _, tt := range tests {
		got := ThumbnailPath(tt.in)
		if got != tt.want {
			t.Errorf("ThumbnailPath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	if got := ThumbnailURLPath("data/resources/abc-123.png"); got != "/data/resources/thumbs/abc-123.jpg" {
		t.Errorf("ThumbnailURLPath = %q", got)
	}
}

func TestThumbDimensions(t *testing.T) {
	tests := []struct {
		w, h, max       int
		wantW, wantH    int
	}{
		{100, 50, 360, 100, 50},
		{800, 600, 360, 360, 270},
		{600, 800, 360, 270, 360},
		{360, 360, 360, 360, 360},
		{1, 2000, 360, 1, 360},
		{2000, 1, 360, 360, 1},
	}
	for _, tt := range tests {
		gw, gh := thumbDimensions(tt.w, tt.h, tt.max)
		if gw != tt.wantW || gh != tt.wantH {
			t.Errorf("thumbDimensions(%d,%d,%d)=%dx%d want %dx%d",
				tt.w, tt.h, tt.max, gw, gh, tt.wantW, tt.wantH)
		}
	}
}

func TestParseThumbRequestPath(t *testing.T) {
	stem, ok := parseThumbRequestPath("thumbs/abc-123.jpg")
	if !ok || stem != "abc-123" {
		t.Fatalf("got %q %v", stem, ok)
	}
	if _, ok := parseThumbRequestPath("thumbs/../abc.jpg"); ok {
		t.Fatal("expected reject traversal")
	}
	if _, ok := parseThumbRequestPath("thumbs/a/b.jpg"); ok {
		t.Fatal("expected reject nested")
	}
	if _, ok := parseThumbRequestPath("other/abc.jpg"); ok {
		t.Fatal("expected reject non-thumb")
	}
	if _, ok := parseThumbRequestPath("thumbs/abc.png"); ok {
		t.Fatal("expected reject non-jpg thumb ext")
	}
}

func TestWriteThumbnailFromBytes_ScalesAndIsJPEG(t *testing.T) {
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	original := "data/resources/test-uuid-1.png"
	if err := os.MkdirAll(path.Dir(original), 0o755); err != nil {
		t.Fatal(err)
	}
	// don't need original file on disk for FromBytes
	data := pngBytes(t, 800, 600)
	if err := WriteThumbnailFromBytes(data, original); err != nil {
		t.Fatalf("WriteThumbnailFromBytes: %v", err)
	}

	thumb := ThumbnailPath(original)
	f, err := os.Open(thumb)
	if err != nil {
		t.Fatalf("open thumb: %v", err)
	}
	defer f.Close()

	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if format != "jpeg" {
		t.Fatalf("format = %q, want jpeg", format)
	}
	if cfg.Width != 360 || cfg.Height != 270 {
		t.Fatalf("size = %dx%d, want 360x270", cfg.Width, cfg.Height)
	}

	// idempotent
	if err := WriteThumbnailFromBytes(data, original); err != nil {
		t.Fatalf("second write: %v", err)
	}
}

func TestWriteThumbnailFromBytes_SmallImageKeepsSize(t *testing.T) {
	origWD, _ := os.Getwd()
	tmp := t.TempDir()
	_ = os.Chdir(tmp)
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	original := "data/resources/small-1.png"
	data := pngBytes(t, 120, 80)
	if err := WriteThumbnailFromBytes(data, original); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(ThumbnailPath(original))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 120 || cfg.Height != 80 {
		t.Fatalf("size = %dx%d, want 120x80", cfg.Width, cfg.Height)
	}
}

func TestEnsureThumbnail_OnDemandFromFile(t *testing.T) {
	origWD, _ := os.Getwd()
	tmp := t.TempDir()
	_ = os.Chdir(tmp)
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	original := "data/resources/ondemand-1.png"
	writeTestPNG(t, original, 500, 500, color.RGBA{0, 128, 255, 255})

	thumb, err := EnsureThumbnail(original, "image/png")
	if err != nil {
		t.Fatalf("EnsureThumbnail: %v", err)
	}
	if thumb != ThumbnailPath(original) {
		t.Fatalf("thumb path = %q", thumb)
	}

	f, err := os.Open(thumb)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := jpeg.Decode(f)
	if err != nil {
		t.Fatalf("jpeg decode: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != 360 || b.Dy() != 360 {
		t.Fatalf("size = %dx%d", b.Dx(), b.Dy())
	}
}

func TestEnsureThumbnail_Concurrent(t *testing.T) {
	origWD, _ := os.Getwd()
	tmp := t.TempDir()
	_ = os.Chdir(tmp)
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	original := "data/resources/concurrent-1.png"
	writeTestPNG(t, original, 400, 200, color.White)

	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := EnsureThumbnail(original, "image/png")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent EnsureThumbnail: %v", err)
		}
	}
	if !thumbExists(ThumbnailPath(original)) {
		t.Fatal("thumb missing after concurrent generate")
	}
}

func TestRemoveThumbnail(t *testing.T) {
	origWD, _ := os.Getwd()
	tmp := t.TempDir()
	_ = os.Chdir(tmp)
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	original := "data/resources/rm-1.png"
	if err := WriteThumbnailFromBytes(pngBytes(t, 50, 50), original); err != nil {
		t.Fatal(err)
	}
	tp := ThumbnailPath(original)
	if !thumbExists(tp) {
		t.Fatal("expected thumb")
	}
	RemoveThumbnail(original)
	if thumbExists(tp) {
		t.Fatal("thumb should be gone")
	}
	// missing is fine
	RemoveThumbnail(original)
}

func TestSaveUploadedFile_CreatesThumbnail(t *testing.T) {
	setupUploadEnv(t)

	data := pngBytes(t, 640, 480)
	header := &multipart.FileHeader{
		Filename: "photo.png",
		Size:     int64(len(data)),
		Header:   make(textproto.MIMEHeader),
	}
	header.Header.Set("Content-Type", "image/png")
	file := &virtualFile{Reader: bytes.NewReader(data)}

	got, err := saveUploadedFile(file, header, "testuser")
	if err != nil {
		t.Fatalf("saveUploadedFile: %v", err)
	}
	if !strings.HasPrefix(got.Type, "image/") {
		t.Fatalf("type = %q", got.Type)
	}
	tp := ThumbnailPath(got.Path)
	if !thumbExists(tp) {
		t.Fatalf("expected thumb at %s", tp)
	}

	RemoveThumbnail(got.Path)
	_ = os.Remove(got.Path)
}

func TestServeThumbnail_OnDemandAndAuth(t *testing.T) {
	setupUploadEnv(t)

	data := pngBytes(t, 400, 300)
	header := &multipart.FileHeader{
		Filename: "shot.png",
		Size:     int64(len(data)),
		Header:   make(textproto.MIMEHeader),
	}
	header.Header.Set("Content-Type", "image/png")
	got, err := saveUploadedFile(&virtualFile{Reader: bytes.NewReader(data)}, header, "testuser")
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	// Remove eager thumb to force on-demand path
	RemoveThumbnail(got.Path)
	if thumbExists(ThumbnailPath(got.Path)) {
		t.Fatal("thumb should be removed before on-demand test")
	}

	stem := resourceStem(got.Path)
	handler := ResourcesHandler(http.NotFoundHandler())

	// Owner can fetch (generates on demand)
	req := httptest.NewRequest(http.MethodGet, "/thumbs/"+stem+".jpg", nil)
	req = req.WithContext(context.WithValue(req.Context(), "user", "testuser"))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("owner status = %d body=%s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Fatalf("Content-Type = %q", ct)
	}
	if !thumbExists(ThumbnailPath(got.Path)) {
		t.Fatal("expected on-demand thumb written")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(rr.Body.Bytes()))
	if err != nil || format != "jpeg" {
		t.Fatalf("response image: format=%s err=%v", format, err)
	}
	if cfg.Width != 360 || cfg.Height != 270 {
		t.Fatalf("served size %dx%d", cfg.Width, cfg.Height)
	}

	// Other user cannot access
	req2 := httptest.NewRequest(http.MethodGet, "/thumbs/"+stem+".jpg", nil)
	req2 = req2.WithContext(context.WithValue(req2.Context(), "user", "other"))
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusNotFound {
		t.Fatalf("other user status = %d, want 404", rr2.Code)
	}

	// Non-image should 404
	txt := []byte("hello")
	th := &multipart.FileHeader{
		Filename: "note.txt",
		Size:     int64(len(txt)),
		Header:   make(textproto.MIMEHeader),
	}
	th.Header.Set("Content-Type", "text/plain")
	txtFile, err := saveUploadedFile(&virtualFile{Reader: bytes.NewReader(txt)}, th, "testuser")
	if err != nil {
		t.Fatalf("text upload: %v", err)
	}
	req3 := httptest.NewRequest(http.MethodGet, "/thumbs/"+resourceStem(txtFile.Path)+".jpg", nil)
	req3 = req3.WithContext(context.WithValue(req3.Context(), "user", "testuser"))
	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusNotFound {
		t.Fatalf("text thumb status = %d, want 404", rr3.Code)
	}
}

func TestPDFThumbnail_EagerAndOnDemand(t *testing.T) {
	setupUploadEnv(t)

	file, header := multipartPDF(t, "hello.pdf", minimalPDF)
	got, err := saveUploadedFile(file, header, "testuser")
	if err != nil {
		// go-fitz may reject the minimal fixture on some platforms; skip then.
		if strings.Contains(err.Error(), "fitz") || strings.Contains(strings.ToLower(err.Error()), "pdf") {
			t.Skipf("pdf upload/thumbnail unavailable: %v", err)
		}
		t.Fatalf("saveUploadedFile: %v", err)
	}
	if got.Type != "application/pdf" {
		t.Fatalf("type = %q", got.Type)
	}

	tp := ThumbnailPath(got.Path)
	if !thumbExists(tp) {
		// Eager may have failed; force on-demand.
		if _, err := EnsureThumbnail(got.Path, got.Type); err != nil {
			t.Skipf("pdf thumbnail generation unavailable: %v", err)
		}
	}
	if !thumbExists(tp) {
		t.Fatal("expected pdf thumb")
	}

	f, err := os.Open(tp)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatalf("decode thumb: %v", err)
	}
	if format != "jpeg" {
		t.Fatalf("format = %q", format)
	}
	if cfg.Width < 1 || cfg.Height < 1 {
		t.Fatalf("invalid size %dx%d", cfg.Width, cfg.Height)
	}
	if cfg.Width > thumbMaxEdge || cfg.Height > thumbMaxEdge {
		t.Fatalf("thumb exceeds max edge: %dx%d", cfg.Width, cfg.Height)
	}

	// On-demand after delete
	RemoveThumbnail(got.Path)
	handler := ResourcesHandler(http.NotFoundHandler())
	stem := resourceStem(got.Path)
	req := httptest.NewRequest(http.MethodGet, "/thumbs/"+stem+".jpg", nil)
	req = req.WithContext(context.WithValue(req.Context(), "user", "testuser"))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("pdf thumb serve status = %d body=%s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Fatalf("Content-Type = %q", ct)
	}
}

func TestDeleteFile_RemovesThumbnail(t *testing.T) {
	setupUploadEnv(t)

	data := pngBytes(t, 80, 80)
	header := &multipart.FileHeader{
		Filename: "del.png",
		Size:     int64(len(data)),
		Header:   make(textproto.MIMEHeader),
	}
	header.Header.Set("Content-Type", "image/png")
	got, err := saveUploadedFile(&virtualFile{Reader: bytes.NewReader(data)}, header, "testuser")
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	tp := ThumbnailPath(got.Path)
	if !thumbExists(tp) {
		t.Fatal("expected thumb before delete")
	}

	req := httptest.NewRequest(http.MethodDelete, "/delete/"+got.ID, nil)
	req = req.WithContext(context.WithValue(req.Context(), "user", "testuser"))
	req.SetPathValue("id", got.ID)
	rr := httptest.NewRecorder()
	deleteFile(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d body=%s", rr.Code, rr.Body.String())
	}
	if thumbExists(tp) {
		t.Fatal("thumb should be deleted")
	}
	if _, err := os.Stat(got.Path); !os.IsNotExist(err) {
		t.Fatalf("original should be deleted, err=%v", err)
	}
}
