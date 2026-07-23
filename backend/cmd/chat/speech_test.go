package chat

import "testing"

func TestStripMarkdownForTTS(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain",
			in:   "Hello world",
			want: "Hello world",
		},
		{
			name: "bold and link",
			in:   "See **docs** at [site](https://example.com)",
			want: "See docs at site",
		},
		{
			name: "code fence",
			in:   "Run:\n```go\nfmt.Println(1)\n```\ndone",
			want: "Run: fmt.Println(1) done",
		},
		{
			name: "heading list",
			in:   "## Title\n- item one\n- item two",
			want: "Title item one item two",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdownForTTS(tt.in)
			if got != tt.want {
				t.Fatalf("stripMarkdownForTTS() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("hello", 10); got != "hello" {
		t.Fatalf("got %q", got)
	}
	if got := truncateRunes("héllo world", 5); got != "héllo" {
		t.Fatalf("got %q", got)
	}
}

func TestClampTTSSpeed(t *testing.T) {
	if got := clampTTSSpeed(1); got != 1 {
		t.Fatalf("got %v", got)
	}
	if got := clampTTSSpeed(0.1); got != 0.25 {
		t.Fatalf("got %v", got)
	}
	if got := clampTTSSpeed(5); got != 4 {
		t.Fatalf("got %v", got)
	}
}

func TestTtsETagStable(t *testing.T) {
	a := ttsETag(1, "hello", "m", "alloy", 1)
	b := ttsETag(1, "hello", "m", "alloy", 1)
	if a != b {
		t.Fatalf("etag not stable: %q vs %q", a, b)
	}
	c := ttsETag(1, "hello!", "m", "alloy", 1)
	if a == c {
		t.Fatal("etag should change when content changes")
	}
	if !etagMatches(a, a) {
		t.Fatal("exact match failed")
	}
	if !etagMatches(`W/`+a, a) {
		t.Fatal("weak match failed")
	}
}
