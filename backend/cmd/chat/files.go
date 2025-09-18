package chat

import (
	"ai-client/cmd/utils"
	"net/http"
	"strings"
)

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

	utils.RespondWithJSON(w, map[string]string{"fileUrl": fileUrl}, http.StatusOK)
}
