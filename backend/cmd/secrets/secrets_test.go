package secrets

import (
	"strings"
	"testing"
)

func TestNormalizeName(t *testing.T) {
	ok, err := NormalizeName("github_token")
	if err != nil || ok != "GITHUB_TOKEN" {
		t.Fatalf("got %q err=%v", ok, err)
	}
	if _, err := NormalizeName("1BAD"); err == nil {
		t.Fatal("expected error for leading digit")
	}
	if _, err := NormalizeName("has-dash"); err == nil {
		t.Fatal("expected error for dash")
	}
	if _, err := NormalizeName("HAS SPACE"); err == nil {
		t.Fatal("expected error for space")
	}
	if _, err := NormalizeName(""); err == nil {
		t.Fatal("expected error for empty")
	}
	if _, err := NormalizeName(strings.Repeat("A", maxSecretNameLen+1)); err == nil {
		t.Fatal("expected error for too long")
	}
}

func TestExpand(t *testing.T) {
	m := map[string]string{
		"GITHUB_TOKEN": "ghp_abc",
		"CF_TOKEN":     "cf_xyz",
	}

	in := `{"headers":{"Authorization":"Bearer $secrets.GITHUB_TOKEN$"}}`
	out, err := Expand(in, m)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"headers":{"Authorization":"Bearer ghp_abc"}}`
	if out != want {
		t.Fatalf("got %s want %s", out, want)
	}

	// Multiple
	in2 := `$secrets.GITHUB_TOKEN$ and $secrets.CF_TOKEN$`
	out2, err := Expand(in2, m)
	if err != nil || out2 != "ghp_abc and cf_xyz" {
		t.Fatalf("got %q err=%v", out2, err)
	}

	// Unknown
	if _, err := Expand(`$secrets.MISSING$`, m); err == nil {
		t.Fatal("expected unknown secret error")
	}

	// No placeholders
	s, err := Expand("plain", m)
	if err != nil || s != "plain" {
		t.Fatal("plain passthrough")
	}

	// Single-pass: value containing placeholder-like text is not re-expanded
	m2 := map[string]string{"A": "$secrets.B$", "B": "real"}
	out3, err := Expand(`x=$secrets.A$`, m2)
	if err != nil {
		t.Fatal(err)
	}
	if out3 != "x=$secrets.B$" {
		t.Fatalf("expected non-recursive expand, got %q", out3)
	}
}

func TestValidateValue(t *testing.T) {
	if err := validateValue(""); err == nil {
		t.Fatal("empty")
	}
	if err := validateValue("ok"); err != nil {
		t.Fatal(err)
	}
	if err := validateValue(strings.Repeat("x", maxSecretValueLen+1)); err == nil {
		t.Fatal("too large")
	}
}


