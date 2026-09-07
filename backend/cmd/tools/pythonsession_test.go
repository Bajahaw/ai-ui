package tools

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Bajahaw/ai-ui/cmd/data"
	logger "github.com/charmbracelet/log"
)

var sessionIDRe = regexp.MustCompile(`(?m)^Session-ID: (\S+)\s*$`)

func setupPythonSessionTest(t *testing.T) string {
	t.Helper()
	seed := os.Getenv("PYTHON_WASM_DIR")
	if seed == "" {
		t.Skip("set PYTHON_WASM_DIR to a dir with python.wasm + python311.zip")
	}
	for _, f := range []string{pythonWasmFile, pythonStdlibFile} {
		if _, err := os.Stat(filepath.Join(seed, f)); err != nil {
			t.Skipf("seed engine missing %s: %v", f, err)
		}
	}
	tmp := t.TempDir()
	t.Chdir(tmp)
	dbPath := path.Join(tmp, "test.db")
	if err := data.InitDataSource(dbPath); err != nil {
		t.Fatalf("InitDataSource: %v", err)
	}
	t.Cleanup(func() { _ = data.DB.Close() })
	if _, err := data.DB.Exec("INSERT INTO Users (username, pass_hash) VALUES (?, ?)", "u1", "x"); err != nil {
		t.Fatal(err)
	}
	if _, err := data.DB.Exec("INSERT INTO Users (username, pass_hash) VALUES (?, ?)", "u2", "x"); err != nil {
		t.Fatal(err)
	}
	SetUpTools(logger.New(os.Stderr), data.DB)
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(seed, "sessions")) })
	return tmp
}

func extractSessionID(t *testing.T, content string) string {
	t.Helper()
	m := sessionIDRe.FindStringSubmatch(content)
	if m == nil {
		t.Fatalf("no Session-ID in output:\n%s", content)
	}
	return m[1]
}

func TestDedupMountName(t *testing.T) {
	sess := &pythonSession{root: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(sess.root, "mnt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := dedupMountName(sess, "a.csv"); got != "a.csv" {
		t.Fatalf("got %q", got)
	}
	if err := os.WriteFile(filepath.Join(sess.root, "mnt", "a.csv"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := dedupMountName(sess, "a.csv"); got != "a (2).csv" {
		t.Fatalf("got %q", got)
	}
	if got := dedupMountName(sess, "../../etc/passwd"); got != "passwd" {
		t.Fatalf("traversal not flattened: %q", got)
	}
	for _, bad := range []string{"", ".", "/"} {
		if got := dedupMountName(sess, bad); got == "" || got == "." || got == "/" {
			t.Fatalf("bad name %q produced %q", bad, got)
		}
	}
}

func TestValidOutName(t *testing.T) {
	for _, ok := range []string{"report.xlsx", "a (2).csv", "x"} {
		if !validOutName(ok) {
			t.Fatalf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", ".", ".hidden", "a/b", `a\b`, "/abs"} {
		if validOutName(bad) {
			t.Fatalf("%q should be invalid", bad)
		}
	}
}

// Live: mount an attachment, read it in the sandbox, ownership enforced.
func TestPythonSessionMountLive(t *testing.T) {
	if os.Getenv("PYTHON_WASM_LIVE") == "" {
		t.Skip("set PYTHON_WASM_LIVE=1")
	}
	setupPythonSessionTest(t)

	f, err := saveBinaryFile([]byte("a,b\n1,2\n"), "", "report.csv", "u1")
	if err != nil {
		t.Fatal(err)
	}
	code := `print(open('/mnt/report.csv').read().strip())`
	args := `{"code":` + strconv.Quote(code) + `,"mount_files":["` + f.ID + `"]}`
	out := executePythonTool(context.Background(), args, "u1")
	t.Logf("output:\n%s", out.Content)
	if !strings.Contains(out.Content, "a,b") {
		t.Fatalf("mounted file not readable:\n%s", out.Content)
	}

	out = executePythonTool(context.Background(), args, "u2")
	if !strings.Contains(out.Content, "not found or not yours") {
		t.Fatalf("expected ownership error, got:\n%s", out.Content)
	}

	out = executePythonTool(context.Background(), `{"code":"print(1)","mount_files":["no-such-id"]}`, "u1")
	if !strings.Contains(out.Content, "not found or not yours") {
		t.Fatalf("expected unknown-id error, got:\n%s", out.Content)
	}
}

// Live: two rounds share /work; /out delivers once as attachment.
func TestPythonSessionRoundsLive(t *testing.T) {
	if os.Getenv("PYTHON_WASM_LIVE") == "" {
		t.Skip("set PYTHON_WASM_LIVE=1")
	}
	setupPythonSessionTest(t)

	r1 := executePythonTool(context.Background(), `{"code":"open('/work/note.txt','w').write('round-one')\nopen('/out/result.txt','w').write('payload-42')\nprint('wrote')"}`, "u1")
	t.Logf("round1:\n%s", r1.Content)
	if !strings.Contains(r1.Content, "Exit-Code: 0") {
		t.Fatalf("round1 failed:\n%s", r1.Content)
	}
	if r1.FileID == "" || !strings.Contains(r1.Content, "result.txt") {
		t.Fatalf("expected /out delivery, got fileID=%q:\n%s", r1.FileID, r1.Content)
	}
	got, err := files.GetByIDs([]string{r1.FileID}, "u1")
	if err != nil || len(got) != 1 {
		t.Fatalf("delivered file not in repo: %v", err)
	}
	if got[0].Name != "result.txt" {
		t.Fatalf("delivered name = %q", got[0].Name)
	}
	data, err := os.ReadFile(got[0].Path)
	if err != nil || string(data) != "payload-42" {
		t.Fatalf("delivered bytes wrong: %q %v", data, err)
	}
	sid := extractSessionID(t, r1.Content)

	r2 := executePythonTool(context.Background(), `{"code":"print(open('/work/note.txt').read())","session_id":"`+sid+`"}`, "u1")
	t.Logf("round2:\n%s", r2.Content)
	if !strings.Contains(r2.Content, "round-one") {
		t.Fatalf("/work did not persist:\n%s", r2.Content)
	}
	if strings.Contains(r2.Content, "Saved file: result.txt") {
		t.Fatalf("result.txt re-delivered:\n%s", r2.Content)
	}
	if extractSessionID(t, r2.Content) != sid {
		t.Fatalf("session id changed between rounds")
	}

	r3 := executePythonTool(context.Background(), `{"code":"print('fresh')","session_id":"deadbeef-nope"}`, "u1")
	if !strings.Contains(r3.Content, "fresh session") {
		t.Fatalf("expected expiry note, got:\n%s", r3.Content)
	}
}

// Live: the xlsx edit story — build a workbook, edit it next round,
// deliver via /out, verify the bytes come back intact.
func TestPythonSessionXlsxRoundTripLive(t *testing.T) {
	if os.Getenv("PYTHON_WASM_LIVE") == "" {
		t.Skip("set PYTHON_WASM_LIVE=1")
	}
	setupPythonSessionTest(t)
	pkgs := `"packages":["et-xmlfile==2.0.0","openpyxl==3.1.5"],"timeout_ms":60000`

	r1 := executePythonTool(context.Background(),
		`{"code":"import openpyxl\nwb = openpyxl.Workbook()\nws = wb.active\nws['A1'] = 'hello'\nwb.save('/work/original.xlsx')\nprint('made')",`+pkgs+`}`, "u1")
	if !strings.Contains(r1.Content, "Exit-Code: 0") {
		t.Fatalf("round1 failed:\n%s", r1.Content)
	}
	sid := extractSessionID(t, r1.Content)

	r2 := executePythonTool(context.Background(),
		`{"code":"import openpyxl\nwb = openpyxl.load_workbook('/work/original.xlsx')\nws = wb.active\nws['B2'] = 41 + 1\nwb.save('/out/edited.xlsx')\nprint('cell:', ws['B2'].value)",`+pkgs+`,"session_id":"`+sid+`"}`, "u1")
	t.Logf("round2:\n%s", r2.Content)
	if !strings.Contains(r2.Content, "cell: 42") {
		t.Fatalf("edit round failed:\n%s", r2.Content)
	}
	if r2.FileID == "" || !strings.Contains(r2.Content, "edited.xlsx") {
		t.Fatalf("expected edited.xlsx delivery:\n%s", r2.Content)
	}
	got, err := files.GetByIDs([]string{r2.FileID}, "u1")
	if err != nil || len(got) != 1 {
		t.Fatalf("delivered file missing: %v", err)
	}
	if got[0].Type != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("wrong mime: %q", got[0].Type)
	}
	raw, err := os.ReadFile(got[0].Path)
	if err != nil || len(raw) < 4 || string(raw[:2]) != "PK" {
		t.Fatalf("delivered bytes are not a zip/xlsx: len=%d err=%v", len(raw), err)
	}
	t.Logf("delivered %s (%d bytes)", got[0].Name, len(raw))
}
