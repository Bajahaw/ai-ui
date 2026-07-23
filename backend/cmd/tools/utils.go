package tools

import (
	"os"
	"path"
	"strings"
	"time"

	fs "github.com/Bajahaw/ai-ui/cmd/files"
	"github.com/google/uuid"
)

func saveGeneratedFile(data []byte, fileName, user string) (fs.File, error) {
	uploadDir := path.Join(".", "data", "resources")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return fs.File{}, err
	}

	ext := path.Ext(fileName)
	id := uuid.New().String()
	diskName := id + ext
	filePath := path.Join(uploadDir, diskName)

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fs.File{}, err
	}

	mimeType := "application/octet-stream"
	switch strings.ToLower(ext) {
	case ".docx":
		mimeType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".pptx":
		mimeType = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".xlsx":
		mimeType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".odt":
		mimeType = "application/vnd.oasis.opendocument.text"
	case ".odp":
		mimeType = "application/vnd.oasis.opendocument.presentation"
	case ".ods":
		mimeType = "application/vnd.oasis.opendocument.spreadsheet"
	case ".png":
		mimeType = "image/png"
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".webp":
		mimeType = "image/webp"
	}

	now := time.Now().Format(time.RFC3339)
	fileData := fs.File{
		ID:         id,
		Name:       fileName,
		Type:       mimeType,
		Size:       int64(len(data)),
		Path:       filePath,
		User:       user,
		CreatedAt:  now,
		UploadedAt: now,
	}

	if err := files.Save(fileData); err != nil {
		_ = os.Remove(filePath)
		return fs.File{}, err
	}

	return fileData, nil
}

