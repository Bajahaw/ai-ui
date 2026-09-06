//go:build !android

package files

import (
	"fmt"

	"github.com/gen2brain/go-fitz"
)

func writePDFThumbnail(srcPath, dstPath string) error {
	// MuPDF is process-global unsafe under concurrent document use.
	fitzMu.Lock()
	defer fitzMu.Unlock()

	doc, err := fitz.New(srcPath)
	if err != nil {
		return err
	}
	defer doc.Close()

	if doc.NumPage() < 1 {
		return fmt.Errorf("pdf has no pages")
	}

	// Low DPI: page-sized raster near thumb resolution (Image() is ~300 DPI).
	img, err := doc.ImageDPI(0, pdfThumbDPI)
	if err != nil {
		return err
	}
	if err := checkThumbDimensions(img.Bounds().Dx(), img.Bounds().Dy()); err != nil {
		return err
	}
	return writeThumbnailImage(img, dstPath)
}
