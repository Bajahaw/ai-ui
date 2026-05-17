package files

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Bajahaw/ai-ui/cmd/providers"

	"github.com/google/uuid"
)

func saveUploadedFile(file multipart.File, handler *multipart.FileHeader, user string) (File, error) {
	const maxUploadSize = 10 << 20 // 10 MB
	defer file.Close()

	fileType, err := detectFileType(file)
	if err != nil {
		return File{}, err
	}

	// Read uploaded data (bounded) into memory first so we can optionally compress
	limitedReader := io.LimitReader(file, maxUploadSize+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return File{}, err
	}

	n := len(data)
	if n > maxUploadSize {
		return File{}, fmt.Errorf("file too large: %d bytes (max %d)", n, maxUploadSize)
	}

	// Basic image compression for image types. If compression produces a smaller
	// payload, use it; otherwise keep original bytes.
	if strings.HasPrefix(fileType, "image/") {
		compressed, err := compressImage(bytes.NewReader(data))
		if err != nil {
			return File{}, err
		}
		data = compressed.Bytes()
	}

	uploadDir := path.Join(".", "data", "resources")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return File{}, err
	}

	fileName := uuid.New().String() + path.Ext(handler.Filename)
	filePath := path.Join(uploadDir, fileName)

	dst, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return File{}, err
	}
	defer dst.Close()

	n, err = dst.Write(data)
	if err != nil {
		_ = os.Remove(filePath)
		return File{}, err
	}
	if n != len(data) {
		_ = os.Remove(filePath)
		return File{}, fmt.Errorf("written bytes mismatch: wrote %d of %d", n, len(data))
	}

	uploadedAt := time.Now()

	fileData := File{
		ID:         uuid.NewString(),
		Name:       handler.Filename,
		Type:       fileType,
		Size:       int64(len(data)),
		Path:       filePath,
		UploadedAt: uploadedAt.Format(time.RFC3339),
	}

	urlPath := strings.TrimPrefix(fileData.Path, ".")

	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}

	if !strings.HasPrefix(urlPath, "/data/resources/") {
		urlPath = "/data/resources/" + strings.TrimPrefix(urlPath, "/")
	}

	// fileUrl := utils.GetServerURL(r) + urlPath

	createdAt := time.Now()
	lastModifiedStr := handler.Header.Get("Last-Modified")
	if lastModifiedStr != "" {
		if ts, err := strconv.ParseInt(lastModifiedStr, 10, 64); err == nil {
			createdAt = time.UnixMilli(ts)
		}
	}

	fileData.User = user
	// fileData.URL = fileUrl
	fileData.CreatedAt = createdAt.Format(time.RFC3339)

	log.Debug("Uploaded file data", "file", fileData)

	// ocr only is for images and other docs
	ocrOnly, _ := settings.Get("attachmentOcrOnly", user)
	if ocrOnly == "true" {
		ocrModel, _ := settings.Get("ocrModel", user)
		fileContent, err := extractFileContent(fileData, ocrModel)
		if err != nil {
			return File{}, err
		}
		fileData.Content = fileContent
		log.Debug("Extracted file content", "content", fileContent)
	}

	err = repo.Save(fileData)
	if err != nil {
		_ = os.Remove(filePath)
		return File{}, err
	}

	return fileData, nil
}

func detectFileType(file multipart.File) (string, error) {
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}
	if n == 0 {
		return "", fmt.Errorf("empty file")
	}

	fileType := http.DetectContentType(buf[:n])
	log.Debug("Uploaded file type", "type", fileType)

	// Reset file read pointer: prefer Seek if available
	if seeker, ok := file.(io.Seeker); ok {
		_, err = seeker.Seek(0, io.SeekStart)
		if err != nil {
			return "", err
		}
	}

	return fileType, nil
}

// compressImage reads an image from r and re-encodes it with conservative
// compression settings. Returns a bytes.Buffer with the compressed image.
// Only stdlib packages are used.
func compressImage(r io.Reader) (*bytes.Buffer, error) {
	img, format, err := image.Decode(r)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	format = strings.ToLower(format)
	switch format {
	case "jpeg", "jpg":
		opts := &jpeg.Options{Quality: 80}
		if err := jpeg.Encode(buf, img, opts); err != nil {
			return nil, err
		}
	case "png":
		enc := png.Encoder{CompressionLevel: png.BestCompression}
		if err := enc.Encode(buf, img); err != nil {
			return nil, err
		}
	case "gif":
		opts := &gif.Options{NumColors: 256}
		if err := gif.Encode(buf, img, opts); err != nil {
			return nil, err
		}
	default:
		// For unknown formats, attempt a JPEG encode as a fallback.
		opts := &jpeg.Options{Quality: 80}
		if err := jpeg.Encode(buf, img, opts); err != nil {
			return nil, err
		}
	}

	return buf, nil
}

// extractFileContent extracts text content from the file at the given URL.
// It sends a request to the OCR service and returns the extracted text.
// currently supports images only. if file content is text, then it is not sent to OCR.
func extractFileContent(file File, model string) (string, error) {
	log.Debug("Extracting content from file", "path", file.Path, "type", file.Type)
	if strings.HasPrefix(file.Type, "text/") {
		fileContent, err := os.ReadFile(file.Path)
		if err != nil {
			log.Error("Error reading text file", "err", err)
			return "", err
		}
		return string(fileContent), nil
	}

	if file.Type == "application/pdf" {
		pages, err := readPDFPages(file.Path, file.ID)
		if err != nil {
			log.Error("Error reading PDF pages", "err", err)
			return "", err
		}

		// indexing all pages individually to allow retrieval of specific pages later.
		err = repo.SavePages(pages)
		if err != nil {
			log.Error("Error saving PDF pages", "err", err)
			return "", err
		}

		return "PDF content page 0: \n\n" + pages[0].Content + "... retrieve rest of content using tools", nil
	}

	if strings.HasPrefix(file.Type, "image/") {
		params := providers.RequestParams{
			Messages: []providers.SimpleMessage{
				{
					Role:    "system",
					Content: "You are an Image recognition and OCR assistant.",
				},
				{
					Role: "user",
					Content: "Extract text content from the given file. " +
						"preserve formatting of code, latex, tables etc. " +
						"as much as possible. If main content is not text, " +
						"provide a detailed description of the image instead.",
					Images: []string{
						file.URL,
					},
				},
			},
			Model: model,
			User:  file.User,
		}

		response, err := provider.SendChatCompletionRequest(params)
		if err != nil || len(response.Content) == 0 {
			return "", err
		}

		return response.Content, nil
	}

	return "", fmt.Errorf("unsupported file type for content extraction: %s", file.Type)
}
