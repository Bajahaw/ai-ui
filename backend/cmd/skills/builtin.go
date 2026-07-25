package skills

import (
	"embed"
	"sort"
	"strings"
	"sync"
)

//go:embed builtin/*.md
var builtinFS embed.FS

const builtinIDPrefix = "builtin:"

var (
	builtinOnce   sync.Once
	builtinCached []*Skill
)

// loadBuiltins parses embedded markdown skills once.
// Frontmatter (YAML-like) is required for a useful description in the model prompt:
//
//	---
//	name: my-skill
//	description: Short when-to-use text for the model.
//	---
//
//	# body...
func loadBuiltins() []*Skill {
	builtinOnce.Do(func() {
		entries, err := builtinFS.ReadDir("builtin")
		if err != nil {
			if log != nil {
				log.Error("Error reading embedded builtin skills", "err", err)
			}
			return
		}
		out := make([]*Skill, 0, len(entries))
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
				continue
			}
			data, err := builtinFS.ReadFile("builtin/" + e.Name())
			if err != nil {
				if log != nil {
					log.Error("Error reading builtin skill file", "file", e.Name(), "err", err)
				}
				continue
			}
			name, desc, content := parseSkillMarkdown(string(data), strings.TrimSuffix(e.Name(), ".md"))
			if name == "" || content == "" {
				continue
			}
			out = append(out, &Skill{
				ID:          builtinIDPrefix + name,
				Name:        name,
				Description: desc,
				Content:     content,
				Builtin:     true,
				Active:      true,
			})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		builtinCached = out
	})
	return builtinCached
}

// parseSkillMarkdown extracts optional frontmatter name/description and body content.
// Falls back to fileStem for name when frontmatter omits it.
func parseSkillMarkdown(raw, fileStem string) (name, description, content string) {
	raw = strings.TrimPrefix(raw, "\uFEFF")
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	trimmed := strings.TrimSpace(raw)
	name = fileStem

	if !strings.HasPrefix(trimmed, "---") {
		return name, "", strings.TrimSpace(raw)
	}

	// Drop opening ---
	rest := strings.TrimPrefix(trimmed, "---")
	rest = strings.TrimPrefix(rest, "\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		// Malformed frontmatter — treat whole file as body
		return name, "", strings.TrimSpace(raw)
	}
	fm := rest[:end]
	body := strings.TrimSpace(rest[end+len("\n---"):])
	// Allow trailing whitespace after closing ---
	body = strings.TrimPrefix(body, "\n")
	body = strings.TrimSpace(body)

	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(strings.ToLower(key))
		val = strings.TrimSpace(val)
		// Strip simple YAML quotes
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		switch key {
		case "name":
			if val != "" {
				name = val
			}
		case "description":
			description = val
		}
	}
	return name, description, body
}

func isBuiltinID(id string) bool {
	return strings.HasPrefix(id, builtinIDPrefix)
}

func getBuiltinByID(id string) (*Skill, bool) {
	if !isBuiltinID(id) {
		return nil, false
	}
	name := strings.TrimPrefix(id, builtinIDPrefix)
	return getBuiltinByName(name)
}

func getBuiltinByName(name string) (*Skill, bool) {
	for _, s := range loadBuiltins() {
		if s.Name == name {
			// Return a shallow copy so callers can set Active without races
			cp := *s
			return &cp, true
		}
	}
	return nil, false
}
