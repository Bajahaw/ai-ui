//go:build android

package files

import "errors"

// errPDFUnsupported is returned by PDF operations in the Android build.
// go-fitz (MuPDF/CGO) does not cross-compile with the NDK, so PDF text
// extraction and PDF thumbnails are disabled until a pdfium-based
// replacement lands. Uploads still work; only content extraction fails.
var errPDFUnsupported = errors.New("PDF processing is not supported in this build yet")

func readDocPages(path string, fileID string) ([]FilePage, error) {
	return nil, errPDFUnsupported
}

func RenderDocPageAsImage(path string, pageNumber int, user string) (File, error) {
	return File{}, errPDFUnsupported
}

func writePDFThumbnail(srcPath, dstPath string) error {
	return errPDFUnsupported
}
