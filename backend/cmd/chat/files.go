package chat

import (
	"ai-client/cmd/data"
	"ai-client/cmd/utils"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type File struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

func upload(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20) // limit to 10MB
	if err != nil {
		log.Error("Error parsing multipart form", "err", err)
		http.Error(w, "Error parsing form data", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		log.Error("Error retrieving file from form data", "err", err)
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}

	defer file.Close()

	filePath, err := utils.SaveUploadedFile(file, handler)
	if err != nil {
		log.Error("Error saving uploaded file", "err", err)
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}

	filePath = strings.TrimPrefix(filePath, ".")

	if !strings.HasPrefix(filePath, "/") {
		filePath = "/" + filePath
	}

	if !strings.HasPrefix(filePath, "/data/resources/") {
		log.Debug("Adjusting file path", "original", filePath)
		filePath = "/data/resources/" + strings.TrimPrefix(filePath, "/")
	}

	fileUrl := utils.GetServerURL(r) + filePath

	fileContent := extractFileContent(fileUrl)

	log.Debug("Extracted file content", "content", fileContent)

	attType := "file"
	if strings.HasPrefix(handler.Header.Get("Content-Type"), "image/") {
		attType = "image"
	} else if strings.HasPrefix(handler.Header.Get("Content-Type"), "video/") {
		attType = "video"
	} else if strings.HasPrefix(handler.Header.Get("Content-Type"), "audio/") {
		attType = "audio"
	}

	fileData := File{
		ID:      uuid.NewString(),
		Type:    attType,
		URL:     fileUrl,
		Content: fileContent,
	}

	err = saveFileData(fileData)
	if err != nil {
		log.Error("Error saving file data", "err", err)
		http.Error(w, "Error saving file data", http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, fileData, http.StatusOK)
}

func getFilesDataByID(fileIDs []string) ([]File, error) {
	if len(fileIDs) == 0 {
		return []File{}, nil
	}

	fileSql := `
	SELECT id, type, url, content
	FROM Files
	WHERE id IN (` + utils.SqlPlaceholders(len(fileIDs)) + `)
	`

	args := make([]interface{}, len(fileIDs))
	for i, id := range fileIDs {
		args[i] = id
	}

	rows, err := data.DB.Query(fileSql, args...)
	if err != nil {
		log.Error("Error querying files", "err", err)
		return []File{}, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var file File
		if err := rows.Scan(
			&file.ID,
			&file.Type,
			&file.URL,
			&file.Content,
		); err != nil {
			log.Error("Error scanning file", "err", err)
			continue
		}
		files = append(files, file)
	}

	return files, nil

}

func saveFileData(file File) error {
	attSql := `INSERT INTO Files (id, type, url, content) VALUES (?, ?, ?, ?)`
	_, err := data.DB.Exec(attSql,
		file.ID,
		file.Type,
		file.URL,
		file.Content,
	)
	if err != nil {
		return err
	}

	return nil
}

// extractFileContent extracts text content from the file at the given URL.
// It sends a request to the OCR service and returns the extracted text.
// currently supports images only. if file content is text, then it is not sent to OCR.
func extractFileContent(fileURL string) string {
	ocrModel, _ := getSetting("ocrModel")

	// ...

	return "extracted text content using: " + ocrModel
}
