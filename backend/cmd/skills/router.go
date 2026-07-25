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
	mux.HandleFunc("GET /{id}", getSkill)
	mux.HandleFunc("POST /save", saveSkill)
	mux.HandleFunc("DELETE /{id}", deleteSkill)

	return http.StripPrefix("/api/skills", auth.Authenticated(mux))
}

func listSkills(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	all := GetAll(user)
	resp := SkillListResponse{
		Skills: make([]SkillResponse, 0, len(all)),
	}
	for _, s := range all {
		resp.Skills = append(resp.Skills, SkillResponse{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
			Builtin:     s.Builtin,
			Active:      s.Active,
		})
	}
	utils.RespondWithJSON(w, resp, http.StatusOK)
}

func getSkill(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	id := r.PathValue("id")
	s, err := GetByID(id, user)
	if err != nil {
		http.Error(w, "Skill not found", http.StatusNotFound)
		return
	}
	utils.RespondWithJSON(w, SkillDetailResponse{
		ID:          s.ID,
		Name:        s.Name,
		Description: s.Description,
		Content:     s.Content,
		Builtin:     s.Builtin,
		Active:      s.Active,
	}, http.StatusOK)
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

	// Builtins are read-only; saving with a builtin id creates/updates a user override.
	if isBuiltinID(req.ID) {
		req.ID = ""
	}

	// Update existing skill when ID is provided; otherwise create new.
	if req.ID != "" {
		skill := &Skill{
			Name:        req.Name,
			Description: req.Description,
			Content:     req.Content,
		}
		if err := repo.Update(req.ID, user, skill); err != nil {
			log.Error("Error updating skill", "err", err)
			http.Error(w, "Error updating skill", http.StatusInternalServerError)
			return
		}
		utils.RespondWithJSON(w, SkillResponse{
			ID: req.ID, Name: req.Name, Description: req.Description,
			Builtin: false, Active: true,
		}, http.StatusOK)
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
		utils.RespondWithJSON(w, SkillResponse{
			Name: req.Name, Description: req.Description,
			Builtin: false, Active: true,
		}, http.StatusCreated)
		return
	}
	utils.RespondWithJSON(w, SkillResponse{
		ID: saved.ID, Name: saved.Name, Description: saved.Description,
		Builtin: false, Active: true,
	}, http.StatusCreated)
}

func deleteSkill(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	id := r.PathValue("id")
	if isBuiltinID(id) {
		http.Error(w, "Built-in skills cannot be deleted; disable them in General Settings", http.StatusBadRequest)
		return
	}
	if err := repo.DeleteByID(id, user); err != nil {
		log.Error("Error deleting skill", "err", err)
		http.Error(w, "Error deleting skill", http.StatusInternalServerError)
		return
	}
	utils.RespondWithJSON(w, map[string]string{"status": "success"}, http.StatusOK)
}
