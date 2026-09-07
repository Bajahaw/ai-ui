package tools

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePackageSpec(t *testing.T) {
	ok := []struct{ in, name, ver string }{
		{"openpyxl", "openpyxl", ""},
		{"openpyxl==3.1.5", "openpyxl", "3.1.5"},
		{"et_xmlfile==2.0.0", "et_xmlfile", "2.0.0"},
		{"zope.interface==6.4", "zope.interface", "6.4"},
		{"pkg==1.0rc1", "pkg", "1.0rc1"},
	}
	for _, tc := range ok {
		n, v, err := parsePackageSpec(tc.in)
		if err != nil || n != tc.name || v != tc.ver {
			t.Fatalf("parsePackageSpec(%q) = %q,%q,%v", tc.in, n, v, err)
		}
	}
	bad := []string{
		"", "==1.0", "name>1.0", "name>=1.0", "name~=1.0", "name[extra]",
		"name==1.0; python_version>'3'", "https://x/y.whl", "../evil", "a/b",
		"name==1.0==2.0", "-lead", "_lead",
	}
	for _, in := range bad {
		if _, _, err := parsePackageSpec(in); err == nil {
			t.Fatalf("parsePackageSpec(%q) should fail", in)
		}
	}
}

func TestSelectPureWheel(t *testing.T) {
	files := []pypiFile{
		{Filename: "pkg-1.0.tar.gz", Packagetype: "sdist"},
		{Filename: "pkg-1.0-cp311-cp311-win_amd64.whl", Packagetype: "bdist_wheel"},
		{Filename: "pkg-1.0-cp311-cp311-manylinux_x86_64.whl", Packagetype: "bdist_wheel"},
		{Filename: "pkg-1.0-cp311-cp311-macosx_11_0_arm64.whl", Packagetype: "bdist_wheel"},
		{Filename: "pkg-1.0-py2.py3-none-any.whl", Packagetype: "bdist_wheel"},
	}
	got, err := selectPureWheel(files)
	if err != nil {
		t.Fatal(err)
	}
	if got.Filename != "pkg-1.0-py2.py3-none-any.whl" {
		t.Fatalf("unexpected pick %q", got.Filename)
	}

	files = append(files, pypiFile{Filename: "pkg-1.0-py3-none-any.whl", Packagetype: "bdist_wheel"})
	got, err = selectPureWheel(files)
	if err != nil {
		t.Fatal(err)
	}
	if got.Filename != "pkg-1.0-py3-none-any.whl" {
		t.Fatalf("py3 wheel should win, got %q", got.Filename)
	}

	if _, err := selectPureWheel(files[:4]); err == nil {
		t.Fatal("expected error when only sdist/platform wheels exist")
	}
}

func makeTestWheel(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractWheelIntoSandbox(t *testing.T) {
	data := makeTestWheel(t, map[string]string{
		"mymod/__init__.py": "X = 1\n",
		"mymod/native.so":   "fake-elf",
		"mymod.dll":         "fake-dll",
	})
	root := t.TempDir()
	budget := int64(pythonMaxExtractedBytes)
	entry, err := extractWheelIntoSandbox(root, preparedPackage{key: "mymod-1.0", data: data}, &budget)
	if err != nil {
		t.Fatal(err)
	}
	if entry != "/packages/mymod-1.0" {
		t.Fatalf("unexpected entry %q", entry)
	}
	if _, err := os.Stat(filepath.Join(root, "packages", "mymod-1.0", "mymod", "__init__.py")); err != nil {
		t.Fatalf("expected extracted file: %v", err)
	}
	for _, native := range []string{"native.so", "mymod.dll"} {
		var p string
		if strings.HasSuffix(native, ".dll") {
			p = filepath.Join(root, "packages", "mymod-1.0", "mymod.dll")
		} else {
			p = filepath.Join(root, "packages", "mymod-1.0", "mymod", native)
		}
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("native file should be skipped: %s", p)
		}
	}
}

func TestExtractWheelZipSlip(t *testing.T) {
	var zeroBudget int64
	for _, evil := range []string{"../../evil.py", "/abs.py", "a/../../../b.py"} {
		data := makeTestWheel(t, map[string]string{evil: "evil"})
		budget := int64(pythonMaxExtractedBytes)
		if _, err := extractWheelIntoSandbox(t.TempDir(), preparedPackage{key: "evil-1.0", data: data}, &budget); err == nil {
			t.Fatalf("zip-slip path %q must be rejected", evil)
		}
	}
	if _, err := extractWheelIntoSandbox(t.TempDir(), preparedPackage{key: "../evil", data: makeTestWheel(t, nil)}, &zeroBudget); err == nil {
		t.Fatal("unsafe package key must be rejected")
	}
}

// Live: real PyPI download + import inside the sandbox (PYTHON_WASM_LIVE=1).
func TestExecutePythonPackagesLive(t *testing.T) {
	if os.Getenv("PYTHON_WASM_LIVE") == "" {
		t.Skip("set PYTHON_WASM_LIVE=1 for live package install")
	}
	out := executePythonTool(context.Background(), `{"code":"import openpyxl\nprint(openpyxl.__version__)\nwb = openpyxl.Workbook()\nws = wb.active\nws['A1'] = 6*7\nprint('cell:', ws['A1'].value)","packages":["et-xmlfile==2.0.0","openpyxl==3.1.5"],"timeout_ms":60000}`, "u1")
	t.Logf("output:\n%s", out.Content)
	if !strings.Contains(out.Content, "Exit-Code: 0") ||
		!strings.Contains(out.Content, "3.1.5") ||
		!strings.Contains(out.Content, "cell: 42") {
		t.Fatalf("live package run failed:\n%s", out.Content)
	}
}

// Live: a C-extension package must be refused with a clear error.
func TestExecutePythonPackagesRejectNative(t *testing.T) {
	if os.Getenv("PYTHON_WASM_LIVE") == "" {
		t.Skip("set PYTHON_WASM_LIVE=1 for live package rejection")
	}
	out := executePythonTool(context.Background(), `{"code":"import numpy","packages":["numpy==2.2.0"],"timeout_ms":60000}`, "u1")
	t.Logf("output:\n%s", out.Content)
	if !strings.Contains(out.Content, "pure-Python") {
		t.Fatalf("expected pure-Python refusal, got:\n%s", out.Content)
	}
}
