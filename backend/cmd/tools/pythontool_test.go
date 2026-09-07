package tools

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBuiltInToolsRegistry(t *testing.T) {
	// Regression guard: every built-in must stay registered with a schema.
	// (execute_python once accidentally replaced http_request here.)
	want := map[string]bool{
		"search_document":    false,
		"read_document_page": false,
		"view_document_page": false,
		"generate_image":     false,
		"read_skill":         false,
		"http_request":       false,
		"execute_python":      false,
	}
	for _, tool := range GetBuiltInTools() {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
		if tool.Name == "" || tool.InputSchema == "" {
			t.Fatalf("built-in tool missing name/schema: %+v", tool)
		}
	}
	for name, found := range want {
		if !found {
			t.Fatalf("built-in tool %q missing from GetBuiltInTools()", name)
		}
	}
}

func TestParseExecutePythonArgs(t *testing.T) {
	p, timeout, err := parseExecutePythonArgs(`{"code":"print(1)"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Code != "print(1)" {
		t.Fatalf("unexpected code %q", p.Code)
	}
	if timeout != pythonDefaultTimeout {
		t.Fatalf("unexpected default timeout %s", timeout)
	}

	if _, _, err := parseExecutePythonArgs(`{"code":""}`); err == nil {
		t.Fatal("expected error for empty code")
	}
	if _, _, err := parseExecutePythonArgs(`{}`); err == nil {
		t.Fatal("expected error for missing code")
	}
	if _, _, err := parseExecutePythonArgs(`not json`); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if _, _, err := parseExecutePythonArgs(`{"code":"x","timeout_ms":50}`); err == nil {
		t.Fatal("expected error for timeout below minimum")
	}
	if _, _, err := parseExecutePythonArgs(`{"code":"x","timeout_ms":999999}`); err == nil {
		t.Fatal("expected error for timeout above maximum")
	}
	if _, _, err := parseExecutePythonArgs(`{"code":"` + strings.Repeat("x", pythonMaxCodeLen+1) + `"}`); err == nil {
		t.Fatal("expected error for oversized code")
	}

	p, timeout, err = parseExecutePythonArgs(`{"code":"print(1)","timeout_ms":5000,"stdin":"hi"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if timeout != 5*time.Second || p.Stdin != "hi" {
		t.Fatalf("unexpected parsed values: %+v %s", p, timeout)
	}
}

func TestFormatPythonResult(t *testing.T) {
	out := formatPythonResult(pythonRunResult{exitCode: 0, stdout: "14\n", stderr: ""}, time.Second)
	if !strings.Contains(out, "Exit-Code: 0") || !strings.Contains(out, "14") {
		t.Fatalf("unexpected format:\n%s", out)
	}
	out = formatPythonResult(pythonRunResult{timedOut: true}, 5*time.Second)
	if !strings.Contains(out, "timed out") {
		t.Fatalf("expected timeout message, got:\n%s", out)
	}
	var cb cappedBuffer
	cb.cap = 4
	if _, err := cb.Write([]byte("123456")); err != nil {
		t.Fatal(err)
	}
	if cb.String() != "1234" || !cb.capped {
		t.Fatalf("cappedBuffer broken: %q capped=%v", cb.String(), cb.capped)
	}
}

func TestExecutePythonToolCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out := executePythonTool(ctx, `{"code":"print(1)"}`, "u1")
	if !strings.Contains(out.Content, "cancelled") {
		t.Fatalf("expected cancellation message, got: %q", out.Content)
	}
}

func TestExecutePythonToolBadArgs(t *testing.T) {
	out := executePythonTool(context.Background(), `{"code":""}`, "u1")
	if !strings.Contains(out.Content, "error") {
		t.Fatalf("expected error message, got: %q", out.Content)
	}
}

// Live end-to-end test against the real WASI engine. Downloads ~18 MiB once
// into PYTHON_WASM_DIR (or ./data/pywasm). Opt-in via PYTHON_WASM_LIVE=1.
func TestExecutePythonToolLive(t *testing.T) {
	if os.Getenv("PYTHON_WASM_LIVE") == "" {
		t.Skip("set PYTHON_WASM_LIVE=1 to run engine download + execution")
	}
	ctx := context.Background()

	out := executePythonTool(ctx, `{"code":"print(2 + 3 * 4)\nfor i in range(3):\n    print('i=', i)"}`, "u1")
	t.Logf("basic output:\n%s", out.Content)
	if !strings.Contains(out.Content, "Exit-Code: 0") || !strings.Contains(out.Content, "14") {
		t.Fatalf("basic execution failed:\n%s", out.Content)
	}

	out = executePythonTool(ctx, `{"code":"import json, math\nprint(json.dumps({'pi': round(math.pi, 4)}))"}`, "u1")
	if !strings.Contains(out.Content, "3.1416") {
		t.Fatalf("stdlib execution failed:\n%s", out.Content)
	}

	out = executePythonTool(ctx, `{"code":"raise ValueError('boom-test')"}`, "u1")
	if !strings.Contains(out.Content, "Exit-Code: 1") || !strings.Contains(out.Content, "boom-test") {
		t.Fatalf("expected traceback with exit 1, got:\n%s", out.Content)
	}

	out = executePythonTool(ctx, `{"code":"print(input() + '!')","stdin":"hello"}`, "u1")
	if !strings.Contains(out.Content, "hello!") {
		t.Fatalf("stdin execution failed:\n%s", out.Content)
	}
}
