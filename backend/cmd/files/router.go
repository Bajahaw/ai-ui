package files

import (
	"net/http"
	"os"
	"strings"

	"github.com/Bajahaw/ai-ui/cmd/auth"
	"github.com/Bajahaw/ai-ui/cmd/utils"
)

func FileHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST 	/upload", upload)
	mux.HandleFunc("GET 	/{id}", getFile)
	mux.HandleFunc("GET 	/all", getAllFiles)
	mux.HandleFunc("DELETE 	/delete/{id}", deleteFile)
	mux.HandleFunc("POST 	/extract-content", extractContent)

	return http.StripPrefix("/api/files", auth.Authenticated(mux))
}

func UserBasedAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := utils.ExtractContextUser(r)

		filename := strings.TrimPrefix(r.URL.Path, "/")
		if filename == "" {
			http.NotFound(w, r)
			return
		}

		expectedPath := "data/resources/" + filename

		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM Files WHERE path = ? AND user = ?", expectedPath, user).Scan(&count)
		if err != nil || count == 0 {
			http.NotFound(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
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

	lastModifiedStr := r.FormValue("lastModified")
	if lastModifiedStr != "" {
		handler.Header.Set("Last-Modified", lastModifiedStr)
	}

	defer file.Close()

	fileData, err := saveUploadedFile(file, handler, user)
	if err != nil {
		log.Error("Error saving uploaded file", "err", err)
		http.Error(w, "Error saving file", http.StatusInternalServerError)
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

func extractContent(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	var req struct {
		FileIDs []string `json:"fileIds"`
	}
	if err := utils.ExtractJSONBody(r, &req); err != nil {
		log.Error("Error parsing request body", "err", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	files, err := repo.GetByIDs(req.FileIDs, user)
	if err != nil {
		log.Error("Error querying files from db", "err", err)
		http.Error(w, "Error retrieving files", http.StatusInternalServerError)
		return
	}

	ocrModel, _ := settings.Get("ocrModel", user)
	updatedFiles := []File{}

	for _, file := range files {
		if file.Content == "" {
			fileContent, err := extractFileContent(file, ocrModel)
			if err != nil {
				log.Error("Error extracting file content", "err", err, "file", file.ID)
				http.Error(w, "Error extracting content: "+err.Error(), http.StatusInternalServerError)
				return
			}
			file.Content = fileContent
			err = repo.UpdateContent(file.ID, user, fileContent)
			if err != nil {
				log.Error("Error saving file with extracted content", "err", err)
			} else {
				updatedFiles = append(updatedFiles, file)
			}
		} else {
			updatedFiles = append(updatedFiles, file)
		}
	}

	utils.RespondWithJSON(w, updatedFiles, http.StatusOK)
}
