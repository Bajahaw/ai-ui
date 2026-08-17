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
	return saveBinaryFile(data, "", fileName, user)
}

// saveBinaryFile stores raw bytes and registers them in the files repo.
// mimeType is optional; when empty it is inferred from the file extension.
func saveBinaryFile(data []byte, mimeType, fileName, user string) (fs.File, error) {
	uploadDir := path.Join(".", "data", "resources")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return fs.File{}, err
	}

	ext := path.Ext(fileName)
	if ext == "" && mimeType != "" {
		ext = extFromMIME(mimeType)
		if !strings.HasSuffix(strings.ToLower(fileName), ext) {
			fileName = fileName + ext
		}
	}
	id := uuid.New().String()
	diskName := id + ext
	filePath := path.Join(uploadDir, diskName)

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fs.File{}, err
	}

	if mimeType == "" {
		mimeType = mimeFromExt(ext)
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

func mimeFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".odt":
		return "application/vnd.oasis.opendocument.text"
	case ".odp":
		return "application/vnd.oasis.opendocument.presentation"
	case ".ods":
		return "application/vnd.oasis.opendocument.spreadsheet"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".pdf":
		return "application/pdf"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	default:
		return "application/octet-stream"
	}
}

func extFromMIME(mimeType string) string {
	mimeType = strings.ToLower(strings.Split(mimeType, ";")[0])
	mimeType = strings.TrimSpace(mimeType)
	switch mimeType {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/wav", "audio/x-wav", "audio/wave":
		return ".wav"
	case "audio/ogg":
		return ".ogg"
	case "application/pdf":
		return ".pdf"
	case "text/plain":
		return ".txt"
	default:
		return ".bin"
	}
}

