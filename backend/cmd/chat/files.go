package chat

import (
	"ai-client/cmd/data"
	"ai-client/cmd/provider"
	"ai-client/cmd/utils"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type File struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Size      int64  `json:"size"`
	Path      string `json:"path"`
	URL       string `json:"url"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
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

	log.Debug("file path", "path", filePath)

	fileRawContent, err := os.ReadFile(filePath)
	if err != nil {
		log.Error("Error reading file", "err", err)
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}

	attType := http.DetectContentType(fileRawContent)
	log.Debug("Detected file content type", "type", attType)

	fileType := "file"
	if strings.HasPrefix(attType, "text/") {
		fileType = "text"
	} else if strings.HasPrefix(attType, "image/") {
		fileType = "image"
	} else if strings.HasPrefix(attType, "video/") {
		fileType = "video"
	} else if strings.HasPrefix(attType, "audio/") {
		fileType = "audio"
	}

	urlPath := strings.TrimPrefix(filePath, ".")

	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}

	if !strings.HasPrefix(urlPath, "/data/resources/") {
		urlPath = "/data/resources/" + strings.TrimPrefix(urlPath, "/")
	}

	fileUrl := utils.GetServerURL(r) + urlPath

	createdAt := time.Now()
	lastModifiedStr := r.FormValue("lastModified")
	if lastModifiedStr != "" {
		if ts, err := strconv.ParseInt(lastModifiedStr, 10, 64); err == nil {
			createdAt = time.UnixMilli(ts)
		}
	}

	fileData := File{
		ID:        uuid.NewString(),
		Name:      handler.Filename,
		Type:      fileType,
		Size:      handler.Size,
		Path:      filePath,
		URL:       fileUrl,
		CreatedAt: createdAt.Format(time.RFC3339),
	}

	log.Debug("Uploaded file data", "file", fileData)

	ocrOnly, _ := getSetting("attachmentOcrOnly")
	if ocrOnly == "true" {
		fileContent, err := extractFileContent(fileData)
		if err != nil {
			log.Error("Error extracting file content", "err", err)
			http.Error(w, "Error extracting file content: "+err.Error(), http.StatusInternalServerError)
			return
		}
		fileData.Content = fileContent
		log.Debug("Extracted file content", "content", fileContent)
	}

	err = saveFileData(fileData)
	if err != nil {
		log.Error("Error saving file data", "err", err)
		http.Error(w, "Error saving file data", http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, fileData, http.StatusOK)
}

func getFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	files, err := getFilesDataByID([]string{id})
	if err != nil || len(files) == 0 {
		log.Warn("File not found", "id", id, "err", err)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	utils.RespondWithJSON(w, files[0], http.StatusOK)
}

func getAllFiles(w http.ResponseWriter, r *http.Request) {
	fileSql := `
	SELECT id, name, type, size, path, url, content, created_at
	FROM Files
	`

	rows, err := data.DB.Query(fileSql)
	if err != nil {
		log.Error("Error querying all files", "err", err)
		http.Error(w, "Error retrieving files", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var file File
		if err := rows.Scan(
			&file.ID,
			&file.Name,
			&file.Type,
			&file.Size,
			&file.Path,
			&file.URL,
			&file.Content,
			&file.CreatedAt,
		); err != nil {
			log.Error("Error scanning file", "err", err)
			continue
		}
		files = append(files, file)
	}

	utils.RespondWithJSON(w, files, http.StatusOK)
}

func getFilesDataByID(fileIDs []string) ([]File, error) {
	if len(fileIDs) == 0 {
		return []File{}, nil
	}

	fileSql := `
	SELECT id, name, type, size, path, url, content, created_at
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
			&file.Name,
			&file.Type,
			&file.Size,
			&file.Path,
			&file.URL,
			&file.Content,
			&file.CreatedAt,
		); err != nil {
			log.Error("Error scanning file", "err", err)
			continue
		}
		files = append(files, file)
	}

	return files, nil
}

func saveFileData(file File) error {
	attSql := `INSERT INTO Files (id, name, type, size, path, url, content, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := data.DB.Exec(attSql,
		file.ID,
		file.Name,
		file.Type,
		file.Size,
		file.Path,
		file.URL,
		file.Content,
		file.CreatedAt,
	)
	if err != nil {
		return err
	}

	return nil
}

// extractFileContent extracts text content from the file at the given URL.
// It sends a request to the OCR service and returns the extracted text.
// currently supports images only. if file content is text, then it is not sent to OCR.
func extractFileContent(file File) (string, error) {
	log.Debug("Extracting content from file", "path", file.Path, "type", file.Type)
	if file.Type == "text" {
		fileContent, err := os.ReadFile(file.Path)
		if err != nil {
			log.Error("Error reading text file", "err", err)
			return "", err
		}
		return string(fileContent), nil
	}

	ocrModel, _ := getSetting("ocrModel")

	params := provider.RequestParams{
		Messages: []provider.SimpleMessage{
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
		Model: ocrModel,
	}

	response, err := providerClient.SendChatCompletionRequest(params)
	if err != nil || len(response.Content) == 0 {
		return "", err
	}

	return response.Content, nil
}
