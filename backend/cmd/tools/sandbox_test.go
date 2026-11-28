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
	if strings.Contains(found.InputSchema, "file_ids") {
		t.Fatalf("schema must not expose file_ids (auto-mount): %s", found.InputSchema)
	}
	if !strings.Contains(found.Description, "sandbox.listFiles()") {
		t.Fatalf("description must document listFiles: %s", found.Description)
	}
	if !strings.Contains(found.Description, "sandbox.loadScript(url)") {
		t.Fatalf("description must document loadScript: %s", found.Description)
	}
	if !strings.Contains(found.Description, "sandbox.screenshot(nameOrBytes, options)") {
		t.Fatalf("description must document screenshot: %s", found.Description)
	}
	if strings.Contains(found.Description, "cdnjs.cloudflare.com") || strings.Contains(found.Description, "cdn.jsdelivr.net") || strings.Contains(found.Description, "file-generation skill") {
		t.Fatalf("tool description must stay limited to the API summary: %s", found.Description)
	}
	if !strings.Contains(found.Description, "return value") {
		t.Fatalf("description must say the return value is reported: %s", found.Description)
	}
	if strings.Contains(found.Description, "HTML or JavaScript") || strings.Contains(found.InputSchema, "HTML") {
		t.Fatalf("sandbox is JS-only; must not advertise HTML mode: %s %s", found.Description, found.InputSchema)
	}
}

func TestBrowserSandboxTool_EchoesReturnValue(t *testing.T) {
	setupSandboxTest(t)

	callID := "call-result"
	done := make(chan providers.ToolOutput, 1)
	go func() {
		done <- browserSandboxTool(context.Background(), callID, `{"code":"return sandbox.listFiles()"}`, "u1", "conv-1")
	}()

	waitForPending(t, callID, "u1")
	if err := completeSandboxWait(callID, "u1", sandboxClientResult{
		OK:     true,
		Result: `["a.xlsx","b.pdf"]`,
	}); err != nil {
		t.Fatal(err)
	}

	out := <-done
	if !strings.Contains(out.Content, "result:\n[\"a.xlsx\",\"b.pdf\"]") {
		t.Fatalf("expected result echoed to model, got: %s", out.Content)
	}
}

func TestBrowserSandboxTool_RejectsLegacyHTMLArg(t *testing.T) {
	out := browserSandboxTool(context.Background(), "call-html", `{"html":"<div></div>"}`, "u1", "conv-1")
	if !strings.Contains(out.Content, "'code' is required") {
		t.Fatalf("html-only args must be rejected, got: %s", out.Content)
	}
}

func TestBrowserSandboxTool_PersistsFiles(t *testing.T) {
	setupSandboxTest(t)

	callID := "call-1"
	done := make(chan providers.ToolOutput, 1)
	go func() {
		done <- browserSandboxTool(context.Background(), callID, `{"code":"console.log(1)"}`, "u1", "conv-1")
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
		done <- browserSandboxTool(ctx, callID, `{"code":"1"}`, "u1", "conv-1")
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

	ch := registerSandboxWait("c1", "u1", "conv-1", time.NewTimer(sandboxTimeout), time.Now().Add(sandboxRunMax))
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
		Result: "42",
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
		if !res.OK || string(res.Files[0].Data) != "abc" || res.Result != "42" {
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

	ch := registerSandboxWait("bound-1", "u1", "conv-1", time.NewTimer(sandboxTimeout), time.Now().Add(sandboxRunMax))
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

func TestSandboxInvokeTool(t *testing.T) {
	endpoint := startTestMCPServer(t)
	setupOAuthTest(t) // after the server so the session cache closes first

	saved := saveServerReq(t, map[string]any{
		"name": "Plain MCP", "endpoint": endpoint, "api_key": "", "headers": map[string]string{},
	})

	callID := "run-invoke"
	registerSandboxWait(callID, "testuser", "conv-1", time.NewTimer(sandboxTimeout), time.Now().Add(sandboxRunMax))
	defer unregisterSandboxWait(callID, "testuser")

	post := func(user, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/sandbox-invoke", strings.NewReader(body))
		r = r.WithContext(context.WithValue(r.Context(), "user", user))
		sandboxInvokeTool(w, r)
		return w
	}
	decode := func(w *httptest.ResponseRecorder) sandboxInvokeResponse {
		t.Helper()
		var res sandboxInvokeResponse
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("decode %q: %v", w.Body.String(), err)
		}
		return res
	}

	// Missing pending run.
	if w := post("testuser", `{"call_id":"missing","name":"echo"}`); w.Code != http.StatusNotFound {
		t.Fatalf("missing pending: got %d", w.Code)
	}
	// Wrong user.
	if w := post("other", `{"call_id":"`+callID+`","name":"echo"}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong user: got %d", w.Code)
	}
	// No recursion from inside the sandbox.
	if res := decode(post("testuser", `{"call_id":"`+callID+`","name":"browser_sandbox","args":{"code":"1"}}`)); res.OK {
		t.Fatalf("recursion must fail: %+v", res)
	}
	// Unknown tool.
	if res := decode(post("testuser", `{"call_id":"`+callID+`","name":"nope"}`)); res.OK {
		t.Fatalf("unknown tool must fail: %+v", res)
	}
	// Disabled tool.
	server, _ := mcps.GetByID(saved.ID, "testuser")
	echo := *server.Tools[0]
	echo.IsEnabled = false
	if err := tools.SaveAll([]*Tool{&echo}); err != nil {
		t.Fatal(err)
	}
	if res := decode(post("testuser", `{"call_id":"`+callID+`","name":"echo","args":{"text":"hi"}}`)); res.OK {
		t.Fatalf("disabled tool must fail: %+v", res)
	}
	echo.IsEnabled = true
	if err := tools.SaveAll([]*Tool{&echo}); err != nil {
		t.Fatal(err)
	}

	// Happy path: the nested call runs through the real MCP session and
	// its content goes back to the code.
	res := decode(post("testuser", `{"call_id":"`+callID+`","name":"echo","args":{"text":"hi"}}`))
	if !res.OK || res.Content != "hi" {
		t.Fatalf("echo: %+v", res)
	}

	// The invoke must not complete or consume the sandbox run itself.
	if err := completeSandboxWait(callID, "testuser", sandboxClientResult{OK: true, Result: "done"}); err != nil {
		t.Fatalf("run must stay pending after invoke: %v", err)
	}
}
