package files

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
	"golang.org/x/sync/singleflight"
)

const (
	thumbMaxEdge     = 360
	thumbJPEGQuality = 80
	thumbSubdir      = "thumbs"
	thumbExt         = ".jpg"
	maxThumbSrcBytes = 50 << 20 // match upload limit
	// Cap decoded pixel count to avoid multi‑hundred‑MB RGBA buffers.
	maxThumbPixels = 16 << 20 // 16 megapixels
	// Render PDF page directly near thumb size (not native ~300 DPI).
	pdfThumbDPI = 72
	// Bound concurrent thumb work (scroll can request dozens at once).
	maxConcurrentThumbs = 2
)

var (
	thumbFlight singleflight.Group
	// serialize writers targeting the same path across flight misses
	thumbPathMu sync.Map // string -> *sync.Mutex
	// Global limit for CPU/RAM heavy generation.
	thumbSem = make(chan struct{}, maxConcurrentThumbs)
	// MuPDF / go-fitz is not safe for concurrent use across documents.
	fitzMu sync.Mutex
)

// ThumbnailPath returns the on-disk path for an original resource path.
// data/resources/{stem}.ext -> data/resources/thumbs/{stem}.jpg
func ThumbnailPath(originalPath string) string {
	stem := resourceStem(originalPath)
	if stem == "" {
		return ""
	}
	return path.Join("data", "resources", thumbSubdir, stem+thumbExt)
}

// ThumbnailURLPath returns the public URL path for a thumbnail.
// data/resources/{stem}.ext -> /data/resources/thumbs/{stem}.jpg
func ThumbnailURLPath(originalPath string) string {
	tp := ThumbnailPath(originalPath)
	if tp == "" {
		return ""
	}
	if !strings.HasPrefix(tp, "/") {
		return "/" + tp
	}
	return tp
}

// RemoveThumbnail deletes the thumbnail for originalPath if present.
func RemoveThumbnail(originalPath string) {
	tp := ThumbnailPath(originalPath)
	if tp == "" {
		return
	}
	_ = os.Remove(tp)
}

// EnsureThumbnail creates the thumbnail if missing. Safe for concurrent callers.
// mimeType selects the generator (raster image vs PDF first page).
func EnsureThumbnail(originalPath, mimeType string) (string, error) {
	if !isThumbnailableMIME(mimeType) {
		return "", fmt.Errorf("type not thumbnailable: %s", mimeType)
	}
	dst := ThumbnailPath(originalPath)
	if dst == "" {
		return "", fmt.Errorf("invalid original path")
	}
	if thumbExists(dst) {
		return dst, nil
	}

	v, err, _ := thumbFlight.Do(dst, func() (any, error) {
		if thumbExists(dst) {
			return dst, nil
		}
		if err := withThumbSlot(func() error {
			return generateThumbnailFile(originalPath, dst, mimeType)
		}); err != nil {
			return "", err
		}
		return dst, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

// WriteThumbnailFromBytes builds a thumbnail from already-loaded image bytes.
// Non-image or decode failures return an error; callers may treat as non-fatal.
func WriteThumbnailFromBytes(data []byte, originalPath string) error {
	dst := ThumbnailPath(originalPath)
	if dst == "" {
		return fmt.Errorf("invalid original path")
	}
	if len(data) == 0 {
		return fmt.Errorf("empty image data")
	}
	if len(data) > maxThumbSrcBytes {
		return fmt.Errorf("image too large for thumbnail")
	}

	return withThumbSlot(func() error {
		mu := mutexFor(dst)
		mu.Lock()
		defer mu.Unlock()

		if thumbExists(dst) {
			return nil
		}
		return decodeAndWriteThumb(data, dst)
	})
}

// WritePDFThumbnail renders page 1 of a PDF on disk into the thumb path.
func WritePDFThumbnail(originalPath string) error {
	dst := ThumbnailPath(originalPath)
	if dst == "" {
		return fmt.Errorf("invalid original path")
	}

	return withThumbSlot(func() error {
		mu := mutexFor(dst)
		mu.Lock()
		defer mu.Unlock()

		if thumbExists(dst) {
			return nil
		}
		return writePDFThumbnail(originalPath, dst)
	})
}

func withThumbSlot(fn func() error) error {
	thumbSem <- struct{}{}
	defer func() { <-thumbSem }()
	return fn()
}

func generateThumbnailFile(srcPath, dstPath, mimeType string) error {
	mu := mutexFor(dstPath)
	mu.Lock()
	defer mu.Unlock()

	if thumbExists(dstPath) {
		return nil
	}

	if isPDFMIME(mimeType) {
		return writePDFThumbnail(srcPath, dstPath)
	}
	if !isImageMIME(mimeType) {
		return fmt.Errorf("unsupported thumbnail type: %s", mimeType)
	}

	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	limited := io.LimitReader(f, maxThumbSrcBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if len(data) > maxThumbSrcBytes {
		return fmt.Errorf("image too large for thumbnail")
	}
	return decodeAndWriteThumb(data, dstPath)
}

func decodeAndWriteThumb(data []byte, dstPath string) error {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return err
	}
	if err := checkThumbDimensions(cfg.Width, cfg.Height); err != nil {
		return err
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}
	return writeThumbnailImage(img, dstPath)
}

func writeThumbnailImage(img image.Image, dstPath string) error {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return fmt.Errorf("invalid image dimensions: %dx%d", w, h)
	}

	tw, th := thumbDimensions(w, h, thumbMaxEdge)
	out := img
	if tw != w || th != h {
		dst := image.NewRGBA(image.Rect(0, 0, tw, th))
		// ApproxBiLinear is much cheaper than CatmullRom for small thumbs.
		draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, bounds, draw.Src, nil)
		out = dst
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}

	tmp := dstPath + ".tmp"
	tf, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}

	encodeErr := jpeg.Encode(tf, out, &jpeg.Options{Quality: thumbJPEGQuality})
	closeErr := tf.Close()
	if encodeErr != nil {
		_ = os.Remove(tmp)
		return encodeErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}

	if err := os.Rename(tmp, dstPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func checkThumbDimensions(w, h int) error {
	if w <= 0 || h <= 0 {
		return fmt.Errorf("invalid image dimensions: %dx%d", w, h)
	}
	// Guard overflow: w*h in int64
	if int64(w)*int64(h) > maxThumbPixels {
		return fmt.Errorf("image too many pixels for thumbnail: %dx%d", w, h)
	}
	return nil
}

func thumbDimensions(w, h, maxEdge int) (int, int) {
	if maxEdge <= 0 {
		return w, h
	}
	if w <= maxEdge && h <= maxEdge {
		return w, h
	}
	if w >= h {
		nw := maxEdge
		nh := h * maxEdge / w
		if nh < 1 {
			nh = 1
		}
		return nw, nh
	}
	nh := maxEdge
	nw := w * maxEdge / h
	if nw < 1 {
		nw = 1
	}
	return nw, nh
}

func resourceStem(originalPath string) string {
	cleaned := strings.ReplaceAll(originalPath, "\\", "/")
	cleaned = path.Clean(cleaned)
	cleaned = strings.TrimPrefix(cleaned, "./")
	// Must remain a direct child of data/resources (no traversal / nesting).
	if path.Dir(cleaned) != "data/resources" {
		return ""
	}
	base := path.Base(cleaned)
	if base == "." || base == "/" || base == "" {
		return ""
	}
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if stem == "" || !isSafeResourceStem(stem) {
		return ""
	}
	return stem
}

func isSafeResourceStem(stem string) bool {
	if stem == "" || stem == "." || stem == ".." {
		return false
	}
	if strings.ContainsAny(stem, `/\`) {
		return false
	}
	for _, r := range stem {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.':
		default:
			return false
		}
	}
	return true
}

func parseThumbRequestPath(rel string) (stem string, ok bool) {
	rel = strings.TrimPrefix(rel, "/")
	rel = strings.ReplaceAll(rel, "\\", "/")
	if !strings.HasPrefix(rel, thumbSubdir+"/") {
		return "", false
	}
	name := strings.TrimPrefix(rel, thumbSubdir+"/")
	if name == "" || strings.Contains(name, "/") {
		return "", false
	}
	if !strings.HasSuffix(strings.ToLower(name), thumbExt) {
		return "", false
	}
	stem = strings.TrimSuffix(name, path.Ext(name))
	if !isSafeResourceStem(stem) {
		return "", false
	}
	return stem, true
}

func thumbExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Size() > 0 && !st.IsDir()
}

func mutexFor(key string) *sync.Mutex {
	v, _ := thumbPathMu.LoadOrStore(key, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func isThumbnailableMIME(mimeType string) bool {
	return isImageMIME(mimeType) || isPDFMIME(mimeType)
}

func isPDFMIME(mimeType string) bool {
	return strings.EqualFold(strings.TrimSpace(mimeType), "application/pdf")
}

func isImageMIME(mimeType string) bool {
	mt := strings.ToLower(mimeType)
	if !strings.HasPrefix(mt, "image/") {
		return false
	}
	// Vector / exotic types we do not rasterize.
	switch mt {
	case "image/svg+xml", "image/svg", "image/heic", "image/heif", "image/avif":
		return false
	default:
		return true
	}
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
