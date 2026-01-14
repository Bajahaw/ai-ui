package files

import (
	"ai-client/cmd/auth"
	"ai-client/cmd/utils"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

func FileHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST 	/upload", upload)
	mux.HandleFunc("GET 	/{id}", getFile)
	mux.HandleFunc("GET 	/all", getAllFiles)
	mux.HandleFunc("DELETE 	/delete/{id}", deleteFile)

	return http.StripPrefix("/api/files", auth.Authenticated(mux))
}

func upload(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
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
		Type:      attType,
		Size:      handler.Size,
		Path:      filePath,
		URL:       fileUrl,
		User:      user,
		CreatedAt: createdAt.Format(time.RFC3339),
	}

	log.Debug("Uploaded file data", "file", fileData)

	ocrOnly, _ := settings.Get("attachmentOcrOnly", user)
	if ocrOnly == "true" {
		ocrModel, _ := settings.Get("ocrModel", user)
		fileContent, err := extractFileContent(fileData, ocrModel)
		if err != nil {
			log.Error("Error extracting file content", "err", err)
			http.Error(w, "Error extracting file content: "+err.Error(), http.StatusInternalServerError)
			return
		}
		fileData.Content = fileContent
		log.Debug("Extracted file content", "content", fileContent)
	}

	err = repo.Save(fileData)
	if err != nil {
		log.Error("Error saving file data", "err", err)
		http.Error(w, "Error saving file data", http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, fileData, http.StatusOK)
}

func getFile(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	id := r.PathValue("id")
	files, err := repo.GetByIDs([]string{id}, user)
	if err != nil || len(files) == 0 {
		log.Warn("File not found", "id", id, "err", err)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	utils.RespondWithJSON(w, files[0], http.StatusOK)
}

func deleteFile(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	id := r.PathValue("id")

	// First, get the file data to delete the physical file
	files, err := repo.GetByIDs([]string{id}, user)
	if err != nil || len(files) == 0 {
		log.Warn("File not found for deletion", "id", id, "err", err)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	err = os.Remove(files[0].Path)
	if err != nil {
		log.Error("Error deleting physical file", "err", err)
		http.Error(w, "Error deleting file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	err = repo.DeleteByID(id, user)
	if err != nil {
		log.Error("Error deleting file record from database", "err", err)
		http.Error(w, "Error deleting file record: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func getAllFiles(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)

	files, err := repo.GetAll(user)
	if err != nil {
		log.Error("Error querying all files", "err", err)
		http.Error(w, "Error retrieving files", http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, files, http.StatusOK)
}
