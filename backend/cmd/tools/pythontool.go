package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Bajahaw/ai-ui/cmd/providers"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
)

// execute_python runs Python 3.11 (CPython compiled to WASI) inside a
// wazero (pure Go, no CGO) WebAssembly sandbox.
//
// Sandbox properties:
//   - No network: the guest only imports wasi_snapshot_preview1; socket
//     calls are unimplemented and fail. No host sockets are granted.
//   - No host filesystem: only a fresh per-call temp dir is mounted as /,
//     containing the stdlib zip and the submitted script. Removed after run.
//   - No persistence: each call gets a new module instance + temp dir.
//   - Bounded output: stdout/stderr are truncated with a note.
//   - Bounded time: wall-clock timeout enforced by the caller. NOTE: a
//     tight infinite loop inside CPython is not always interruptible by
//     context cancellation (observed with wazero v1.12 compiler engine),
//     so on timeout we return an error to the caller while the abandoned
//     guest may keep a goroutine busy until process exit. Never hold shared
//     locks across a run for this reason.
//
// Engine provisioning: python.wasm + python311.zip are downloaded once from
// pinned URLs (SHA256 verified) into ./data/pywasm (gitignored, same
// convention as ./data/resources). Override with PYTHON_WASM_DIR.

const (
	pythonWasmURL      = "https://github.com/wch/python-wasm-demo/raw/main/bin/python-3.11.1.wasm"
	pythonWasmSHA256   = "32d0f7e159c20af53ea036ff25b8b005e3edfd653b88ea6a4c2186a01ca2f163"
	pythonStdlibURL    = "https://raw.githubusercontent.com/wch/python-wasm-demo/main/usr/local/lib/python311.zip"
	pythonStdlibSHA256 = "a08d8a226fef6c39b1b1c324f527c79c62048355d66c13b6eCCeb870380d8d02"

	pythonWasmFile   = "python.wasm"
	pythonStdlibFile = "python311.zip"

	pythonMaxCodeLen   = 32 << 10 // 32 KiB of source
	pythonMaxStdinLen  = 32 << 10 // 32 KiB of stdin
	pythonMaxOutputLen = 64 << 10 // 64 KiB captured per stream
	pythonMaxEngineDL  = 32 << 20 // 32 MiB per engine file download cap

	pythonDefaultTimeout = 15 * time.Second
	pythonMinTimeout     = 1 * time.Second
	pythonMaxTimeout     = 60 * time.Second
)

type executePythonParams struct {
	Code      string `json:"code"`
	TimeoutMs int    `json:"timeout_ms"`
	Stdin     string `json:"stdin"`
}

func parseExecutePythonArgs(args string) (*executePythonParams, time.Duration, error) {
	var p executePythonParams
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return nil, 0, fmt.Errorf("error decoding arguments: %w", err)
	}
	if strings.TrimSpace(p.Code) == "" {
		return nil, 0, errors.New("error: 'code' parameter is required and must not be empty")
	}
	if len(p.Code) > pythonMaxCodeLen {
		return nil, 0, fmt.Errorf("error: 'code' exceeds maximum length of %d chars (%d given)", pythonMaxCodeLen, len(p.Code))
	}
	if len(p.Stdin) > pythonMaxStdinLen {
		return nil, 0, fmt.Errorf("error: 'stdin' exceeds maximum length of %d chars (%d given)", pythonMaxStdinLen, len(p.Stdin))
	}
	timeout := pythonDefaultTimeout
	if p.TimeoutMs != 0 {
		timeout = time.Duration(p.TimeoutMs) * time.Millisecond
		if timeout < pythonMinTimeout || timeout > pythonMaxTimeout {
			return nil, 0, fmt.Errorf("error: 'timeout_ms' must be between %d and %d", int(pythonMinTimeout/time.Millisecond), int(pythonMaxTimeout/time.Millisecond))
		}
	}
	return &p, timeout, nil
}

// cappedBuffer captures up to cap bytes, then discards and records truncation.
type cappedBuffer struct {
	buf  bytes.Buffer
	cap  int
	capped bool
}

func (w *cappedBuffer) Write(p []byte) (int, error) {
	if w.buf.Len() >= w.cap {
		w.capped = true
		return len(p), nil
	}
	room := w.cap - w.buf.Len()
	if len(p) > room {
		w.buf.Write(p[:room])
		w.capped = true
		return len(p), nil
	}
	return w.buf.Write(p)
}

func (w *cappedBuffer) String() string { return w.buf.String() }

type pythonEngine struct {
	dir       string
	wasmBytes []byte
	stdlib    []byte
	runtime   wazero.Runtime
	compiled  wazero.CompiledModule
}

var (
	pythonEngineMu  sync.Mutex
	pythonEngineBox *pythonEngine
)

func pythonEngineDir() string {
	if d := strings.TrimSpace(os.Getenv("PYTHON_WASM_DIR")); d != "" {
		return d
	}
	return filepath.Join(".", "data", "pywasm")
}

func downloadVerified(url, wantSHA string, maxBytes int64) ([]byte, error) {
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("downloading engine: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading engine: unexpected status %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading engine download: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("engine download exceeds %d bytes", maxBytes)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != strings.ToLower(wantSHA) {
		return nil, errors.New("engine download failed integrity check (sha256 mismatch)")
	}
	return data, nil
}

func ensurePythonEngine(ctx context.Context) (*pythonEngine, error) {
	pythonEngineMu.Lock()
	defer pythonEngineMu.Unlock()
	if pythonEngineBox != nil {
		return pythonEngineBox, nil
	}

	dir := pythonEngineDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating engine dir: %w", err)
	}
	loadOrFetch := func(name, url, sha string) ([]byte, error) {
		p := filepath.Join(dir, name)
		if b, err := os.ReadFile(p); err == nil {
			sum := sha256.Sum256(b)
			if hex.EncodeToString(sum[:]) == strings.ToLower(sha) {
				return b, nil
			}
			if log != nil {
				log.Warn("Cached python engine failed integrity check, re-downloading", "file", p)
			}
		}
		b, err := downloadVerified(url, sha, pythonMaxEngineDL)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(p, b, 0o644); err != nil {
			return nil, fmt.Errorf("caching engine file: %w", err)
		}
		return b, nil
	}

	wasmBytes, err := loadOrFetch(pythonWasmFile, pythonWasmURL, pythonWasmSHA256)
	if err != nil {
		return nil, err
	}
	stdlib, err := loadOrFetch(pythonStdlibFile, pythonStdlibURL, pythonStdlibSHA256)
	if err != nil {
		return nil, err
	}

	r := wazero.NewRuntime(ctx)
	wasi_snapshot_preview1.MustInstantiate(ctx, r)
	compiled, err := r.CompileModule(ctx, wasmBytes)
	if err != nil {
		r.Close(ctx)
		return nil, fmt.Errorf("compiling python wasm: %w", err)
	}

	pythonEngineBox = &pythonEngine{dir: dir, wasmBytes: wasmBytes, stdlib: stdlib, runtime: r, compiled: compiled}
	return pythonEngineBox, nil
}

type pythonRunResult struct {
	exitCode int
	stdout   string
	stderr   string
	outCut   bool
	errCut   bool
	timedOut bool
	runErr   error
}

func runPythonSandbox(ctx context.Context, eng *pythonEngine, code, stdin string, timeout time.Duration) pythonRunResult {
	// Fresh temp root per call: <root>/usr/local/lib/python311.zip + <root>/tmp/main.py
	root, err := os.MkdirTemp("", "pyexec-*")
	if err != nil {
		return pythonRunResult{runErr: fmt.Errorf("creating sandbox dir: %w", err)}
	}
	defer os.RemoveAll(root)

	libDir := filepath.Join(root, "usr", "local", "lib")
	tmpDir := filepath.Join(root, "tmp")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		return pythonRunResult{runErr: err}
	}
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return pythonRunResult{runErr: err}
	}
	if err := os.WriteFile(filepath.Join(libDir, pythonStdlibFile), eng.stdlib, 0o644); err != nil {
		return pythonRunResult{runErr: err}
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "main.py"), []byte(code), 0o644); err != nil {
		return pythonRunResult{runErr: err}
	}

	stdout := &cappedBuffer{cap: pythonMaxOutputLen}
	stderr := &cappedBuffer{cap: pythonMaxOutputLen}

	modCfg := wazero.NewModuleConfig().
		WithArgs("python", "/tmp/main.py").
		WithEnv("PYTHONPATH", "/usr/local/lib/"+pythonStdlibFile).
		WithEnv("PYTHONHOME", "/usr/local/lib/"+pythonStdlibFile).
		WithFSConfig(wazero.NewFSConfig().WithDirMount(root, "/")).
		WithStdout(stdout).
		WithStderr(stderr)
	if stdin != "" {
		modCfg = modCfg.WithStdin(strings.NewReader(stdin))
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type outcome struct {
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		// InstantiateModule runs the WASI _start function to completion.
		_, err := eng.runtime.InstantiateModule(runCtx, eng.compiled, modCfg)
		done <- outcome{err: err}
	}()

	var runErr error
	timedOut := false
	select {
	case o := <-done:
		runErr = o.err
	case <-runCtx.Done():
		timedOut = true
		if ctx.Err() != nil {
			return pythonRunResult{stdout: stdout.String(), stderr: stderr.String(), outCut: stdout.capped, errCut: stderr.capped, timedOut: false, runErr: errors.New("tool call was cancelled")}
		}
	}

	res := pythonRunResult{
		stdout: stdout.String(), stderr: stderr.String(),
		outCut: stdout.capped, errCut: stderr.capped, timedOut: timedOut,
	}
	if timedOut {
		return res
	}
	if runErr == nil {
		res.exitCode = 0
		return res
	}
	var exitErr *sys.ExitError
	if errors.As(runErr, &exitErr) {
		res.exitCode = int(exitErr.ExitCode())
		return res
	}
	res.runErr = runErr
	return res
}

func formatPythonResult(res pythonRunResult, timeout time.Duration) string {
	var b strings.Builder
	if res.timedOut {
		fmt.Fprintf(&b, "Error: execution timed out after %s.\n", timeout)
		b.WriteString("Note: the sandboxed process was abandoned; it had no network or host filesystem access.\n")
		return b.String()
	}
	if res.runErr != nil {
		fmt.Fprintf(&b, "Error: sandbox execution failed: %v\n", res.runErr)
		return b.String()
	}
	fmt.Fprintf(&b, "Exit-Code: %d\n", res.exitCode)
	b.WriteString("Stdout:\n")
	if res.stdout == "" {
		b.WriteString("(empty)\n")
	} else {
		b.WriteString(res.stdout)
		if !strings.HasSuffix(res.stdout, "\n") {
			b.WriteString("\n")
		}
	}
	if res.outCut {
		fmt.Fprintf(&b, "[stdout truncated to %d bytes]\n", pythonMaxOutputLen)
	}
	b.WriteString("Stderr:\n")
	if res.stderr == "" {
		b.WriteString("(empty)\n")
	} else {
		b.WriteString(res.stderr)
		if !strings.HasSuffix(res.stderr, "\n") {
			b.WriteString("\n")
		}
	}
	if res.errCut {
		fmt.Fprintf(&b, "[stderr truncated to %d bytes]\n", pythonMaxOutputLen)
	}
	return b.String()
}

func executePythonTool(ctx context.Context, args string) providers.ToolOutput {
	params, timeout, err := parseExecutePythonArgs(args)
	if err != nil {
		return providers.ToolOutput{Content: err.Error()}
	}

	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return providers.ToolOutput{Content: "Tool call was cancelled."}
	}

	eng, err := ensurePythonEngine(ctx)
	if err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("Python sandbox unavailable: %v", err)}
	}

	res := runPythonSandbox(ctx, eng, params.Code, params.Stdin, timeout)
	return providers.ToolOutput{Content: formatPythonResult(res, timeout)}
}
