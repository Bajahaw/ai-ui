package skills

type Skill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Content     string `json:"content,omitempty"`
	User        string `json:"-"`
	// Builtin is true for skills shipped with the app (embedded).
	Builtin bool `json:"builtin,omitempty"`
	// Active is false when a builtin is present but disabled for this user
	// (global enableBuiltinSkills toggle is off). User skills are always active.
	Active bool `json:"active"`
}

type SkillResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Builtin     bool   `json:"builtin"`
	Active      bool   `json:"active"`
}

type SkillListResponse struct {
	Skills []SkillResponse `json:"skills"`
}

type SkillDetailResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Builtin     bool   `json:"builtin"`
	Active      bool   `json:"active"`
}

type SkillRequest struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
}
