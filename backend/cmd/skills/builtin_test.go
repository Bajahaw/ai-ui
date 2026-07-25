package skills

import (
	"strings"
	"testing"
)

func TestParseSkillMarkdown_Frontmatter(t *testing.T) {
	raw := "---\nname: demo\ndescription: When to use demo.\n---\n\n# Demo\n\nBody here.\n"
	name, desc, content := parseSkillMarkdown(raw, "fallback")
	if name != "demo" {
		t.Fatalf("name: got %q", name)
	}
	if desc != "When to use demo." {
		t.Fatalf("description: got %q", desc)
	}
	if !strings.Contains(content, "# Demo") || !strings.Contains(content, "Body here.") {
		t.Fatalf("content: got %q", content)
	}
}

func TestParseSkillMarkdown_NoFrontmatter(t *testing.T) {
	raw := "# only body\n"
	name, desc, content := parseSkillMarkdown(raw, "file-stem")
	if name != "file-stem" {
		t.Fatalf("name: got %q", name)
	}
	if desc != "" {
		t.Fatalf("description: got %q", desc)
	}
	if !strings.Contains(content, "only body") {
		t.Fatalf("content: got %q", content)
	}
}

func TestLoadBuiltins(t *testing.T) {
	list := loadBuiltins()
	if len(list) < 2 {
		t.Fatalf("expected at least 2 builtins, got %d", len(list))
	}
	byName := map[string]*Skill{}
	for _, s := range list {
		byName[s.Name] = s
		if !s.Builtin {
			t.Errorf("%s: expected Builtin", s.Name)
		}
		if !strings.HasPrefix(s.ID, builtinIDPrefix) {
			t.Errorf("%s: bad id %q", s.Name, s.ID)
		}
		if s.Description == "" {
			t.Errorf("%s: missing description (needed for model catalog)", s.Name)
		}
		if s.Content == "" {
			t.Errorf("%s: empty content", s.Name)
		}
	}
	for _, want := range []string{"good-widgets", "match-stats-widget"} {
		if _, ok := byName[want]; !ok {
			t.Errorf("missing builtin %q", want)
		}
	}
}
