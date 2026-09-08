package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Bajahaw/ai-ui/cmd/data"
	"github.com/Bajahaw/ai-ui/cmd/providers"
	logger "github.com/charmbracelet/log"
)

func setupSandboxTest(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Chdir(tmp)

	dbPath := tmp + "/test.db"
	if err := data.InitDataSource(dbPath); err != nil {
		t.Fatalf("InitDataSource: %v", err)
	}
	t.Cleanup(func() { _ = data.DB.Close() })

	if _, err := data.DB.Exec("INSERT INTO Users (username, pass_hash) VALUES (?, ?)", "u1", "x"); err != nil {
		t.Fatal(err)
	}
	SetUpTools(logger.New(os.Stderr), data.DB)
}

func TestGetBuiltInTools_IncludesSandbox(t *testing.T) {
	var found *Tool
	for _, tool := range GetBuiltInTools() {
		if tool.Name == "browser_sandbox" {
			found = tool
			break
		}
	}
	if found == nil {
		t.Fatal("missing browser_sandbox")
	}
	if !found.RequireApproval {
		t.Fatal("browser_sandbox must require approval")
	}
	if found.IsEnabled {
		t.Fatal("browser_sandbox should be disabled by default")
	}
	if !strings.Contains(found.InputSchema, `"code"`) {
		t.Fatalf("schema missing code: %s", found.InputSchema)
	}
}

func TestBrowserSandboxTool_PersistsFiles(t *testing.T) {
	setupSandboxTest(t)

	callID := "call-1"
	done := make(chan providers.ToolOutput, 1)
	go func() {
		done <- browserSandboxTool(context.Background(), callID, `{"code":"console.log(1)"}`, "u1")
	}()

	waitForPending(t, callID, "u1")
	err := completeSandboxWait(callID, "u1", sandboxClientResult{
		OK:   true,
		Logs: []string{"1"},
		Files: []sandboxClientFile{{
			Name: "note.txt",
			MIME: "text/plain",
			Data: []byte("hello"),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	out := <-done
	if out.FileID == "" {
		t.Fatal("expected FileID")
	}
	if !strings.Contains(out.Content, "ok: true") || !strings.Contains(out.Content, "note.txt") {
		t.Fatalf("content: %s", out.Content)
	}
	saved, err := files.GetByIDs([]string{out.FileID}, "u1")
	if err != nil || len(saved) != 1 {
		t.Fatalf("saved file: %v %v", saved, err)
	}
	if saved[0].Name != "note.txt" {
		t.Fatalf("name: %s", saved[0].Name)
	}
}

func TestBrowserSandboxTool_Cancel(t *testing.T) {
	setupSandboxTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	callID := "call-cancel"
	done := make(chan providers.ToolOutput, 1)
	go func() {
		done <- browserSandboxTool(ctx, callID, `{"code":"1"}`, "u1")
	}()
	waitForPending(t, callID, "u1")
	cancel()
	out := <-done
	if !strings.Contains(out.Content, "cancelled") {
		t.Fatalf("got %q", out.Content)
	}
}

func TestSubmitSandboxResult_AuthAndPending(t *testing.T) {
	setupSandboxTest(t)

	req := httptest.NewRequest(http.MethodPost, "/sandbox-result", bytes.NewBufferString(`{"call_id":"missing","ok":true}`))
	req = req.WithContext(context.WithValue(req.Context(), "user", "u1"))
	w := httptest.NewRecorder()
	submitSandboxResult(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("missing pending: got %d", w.Code)
	}

	ch := registerSandboxWait("c1", "u1")
	req = httptest.NewRequest(http.MethodPost, "/sandbox-result", bytes.NewBufferString(`{"call_id":"c1","ok":true}`))
	req = req.WithContext(context.WithValue(req.Context(), "user", "other"))
	w = httptest.NewRecorder()
	submitSandboxResult(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong user: got %d", w.Code)
	}

	body, _ := json.Marshal(sandboxResultRequest{
		CallID: "c1",
		OK:     true,
		Logs:   []string{"hi"},
		Files: []sandboxResultFile{{
			Name: "a.bin",
			Data: base64.StdEncoding.EncodeToString([]byte("abc")),
		}},
	})
	req = httptest.NewRequest(http.MethodPost, "/sandbox-result", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), "user", "u1"))
	w = httptest.NewRecorder()
	submitSandboxResult(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("happy path: got %d %s", w.Code, w.Body.String())
	}
	select {
	case res := <-ch:
		if !res.OK || string(res.Files[0].Data) != "abc" {
			t.Fatalf("result: %+v", res)
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive result")
	}
}

func TestSanitizeSandboxFilename(t *testing.T) {
	if got := sanitizeSandboxFilename(`..\..\evil.xlsx`); got != "evil.xlsx" {
		t.Fatalf("got %q", got)
	}
	if got := sanitizeSandboxFilename(""); got != "output.bin" {
		t.Fatalf("got %q", got)
	}
	if got := sanitizeSandboxFilename("a\x00b.txt"); got != "ab.txt" {
		t.Fatalf("control chars: got %q", got)
	}
	if got := sanitizeSandboxFilename(strings.Repeat("a", 300) + ".txt"); len([]rune(got)) > sandboxMaxNameRunes {
		t.Fatalf("length cap: got %d runes", len([]rune(got)))
	}
}

func TestSubmitSandboxResult_MethodAndBounds(t *testing.T) {
	setupSandboxTest(t)

	req := httptest.NewRequest(http.MethodGet, "/sandbox-result", nil)
	req = req.WithContext(context.WithValue(req.Context(), "user", "u1"))
	w := httptest.NewRecorder()
	submitSandboxResult(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method guard: got %d", w.Code)
	}

	ch := registerSandboxWait("bound-1", "u1")
	longErr := strings.Repeat("e", sandboxMaxErrorRunes+100)
	manyLogs := make([]string, sandboxMaxLogLines+50)
	for i := range manyLogs {
		manyLogs[i] = "ok"
	}
	body, _ := json.Marshal(sandboxResultRequest{
		CallID: "bound-1",
		OK:     false,
		Error:  longErr,
		Logs:   manyLogs,
	})
	req = httptest.NewRequest(http.MethodPost, "/sandbox-result", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), "user", "u1"))
	w = httptest.NewRecorder()
	submitSandboxResult(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("bounded submit: got %d %s", w.Code, w.Body.String())
	}
	select {
	case res := <-ch:
		if len(res.Logs) > sandboxMaxLogLines {
			t.Fatalf("logs not capped: %d", len(res.Logs))
		}
		if len([]rune(res.Error)) > sandboxMaxErrorRunes {
			t.Fatalf("error not capped: %d", len([]rune(res.Error)))
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive bounded result")
	}
}

func TestTruncateRunes(t *testing.T) {
	if truncateRunes("abc", 5) != "abc" {
		t.Fatal("short string changed")
	}
	if got := truncateRunes("abcdef", 3); got != "abc" {
		t.Fatalf("got %q", got)
	}
}

func waitForPending(t *testing.T, callID, user string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		sandboxWait.mu.Lock()
		p, ok := sandboxWait.pending[callID]
		sandboxWait.mu.Unlock()
		if ok && p.User == user {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for pending sandbox")
}
