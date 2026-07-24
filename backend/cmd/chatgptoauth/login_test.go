package chatgptoauth

import (
	"testing"
)

func TestParseCallbackInput(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		wantCode  string
		wantState string
		wantErr   bool
	}{
		{
			name:      "full localhost url",
			raw:       "http://localhost:1455/auth/callback?code=abc123&state=xyz789",
			wantCode:  "abc123",
			wantState: "xyz789",
		},
		{
			name:      "path only",
			raw:       "/auth/callback?code=c1&state=s1",
			wantCode:  "c1",
			wantState: "s1",
		},
		{
			name:      "bare query",
			raw:       "code=c2&state=s2",
			wantCode:  "c2",
			wantState: "s2",
		},
		{
			name:      "query with leading ?",
			raw:       "?code=c3&state=s3",
			wantCode:  "c3",
			wantState: "s3",
		},
		{
			name:    "empty",
			raw:     "   ",
			wantErr: true,
		},
		{
			name:    "no oauth params",
			raw:     "http://localhost:1455/auth/callback",
			wantErr: false, // parses but code/state empty — completeFromQuery rejects
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, err := parseCallbackInput(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantCode != "" && q.Get("code") != tc.wantCode {
				t.Errorf("code: got %q want %q", q.Get("code"), tc.wantCode)
			}
			if tc.wantState != "" && q.Get("state") != tc.wantState {
				t.Errorf("state: got %q want %q", q.Get("state"), tc.wantState)
			}
		})
	}
}

func TestCompleteFromCallbackURL_unknownState(t *testing.T) {
	m := NewLoginManager()
	err := m.CompleteFromCallbackURL("http://localhost:1455/auth/callback?code=x&state=not-pending")
	if err == nil {
		t.Fatal("expected error for unknown state")
	}
}
