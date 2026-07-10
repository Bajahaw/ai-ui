package skills

import (
	"net/http"

	"github.com/Bajahaw/ai-ui/cmd/auth"
	"github.com/Bajahaw/ai-ui/cmd/utils"
	"github.com/google/uuid"
)

func Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /all", listSkills)
	mux.HandleFunc("POST /save", saveSkill)
	mux.HandleFunc("DELETE /{id}", deleteSkill)

	return http.StripPrefix("/api/skills", auth.Authenticated(mux))
}

func listSkills(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	all := repo.GetAll(user)
	resp := SkillListResponse{
		Skills: make([]SkillResponse, 0, len(all)),
	}
	for _, s := range all {
		resp.Skills = append(resp.Skills, SkillResponse{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
		})
	}
	utils.RespondWithJSON(w, resp, http.StatusOK)
}

func saveSkill(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	var req SkillRequest
	if err := utils.ExtractJSONBody(r, &req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Content == "" {
		http.Error(w, "Name and content are required", http.StatusBadRequest)
		return
	}

	skill := &Skill{
		ID:          uuid.NewString(),
		Name:        req.Name,
		Description: req.Description,
		Content:     req.Content,
		User:        user,
	}
	if err := repo.Save(skill); err != nil {
		log.Error("Error saving skill", "err", err)
		http.Error(w, "Error saving skill", http.StatusInternalServerError)
		return
	}

	// Re-fetch to get the canonical ID (upsert may have kept an existing row).
	saved, err := repo.GetByName(req.Name, user)
	if err != nil {
		utils.RespondWithJSON(w, SkillResponse{Name: req.Name, Description: req.Description}, http.StatusCreated)
		return
	}
	utils.RespondWithJSON(w, SkillResponse{ID: saved.ID, Name: saved.Name, Description: saved.Description}, http.StatusCreated)
}

func deleteSkill(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	id := r.PathValue("id")
	if err := repo.DeleteByID(id, user); err != nil {
		log.Error("Error deleting skill", "err", err)
		http.Error(w, "Error deleting skill", http.StatusInternalServerError)
		return
	}
	utils.RespondWithJSON(w, map[string]string{"status": "success"}, http.StatusOK)
}
