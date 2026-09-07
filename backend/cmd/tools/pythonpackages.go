package tools

import (
	"archive/zip"
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
	"regexp"
	"strings"
	"time"
)

// Backend-provisioned PyPI packages for the Python WASM sandbox.
//
// The guest has no network and no TLS, so it cannot fetch packages itself.
// Instead the Go host downloads pure-Python wheels from PyPI over real
// HTTPS, verifies the SHA256 recorded by PyPI, extracts them into the
// per-call sandbox FS, and adds them to PYTHONPATH. Same architecture as
// Pyodide's micropip (host fetches, guest installs), with Go as the host.
//
// Hard limits (this is what keeps it safe):
//   - Only *pure-Python* wheels (filename ends "-none-any.whl"). Anything
//     needing C extensions has a platform tag and is rejected with a clear
//     error naming the package.
//   - Specs are strict "name" or "name==version". No ranges, extras, URLs,
//     or direct references (those would let the model pick arbitrary hosts).
//   - No dependency resolution: the caller must list ALL packages including
//     transitive deps (documented in the tool schema). The agent iterates on
//     ImportError, which names the missing module.
//   - Zip-slip guarded extraction, .so/.pyd/.dll skipped, byte caps.

const (
	pythonPyPIBase          = "https://pypi.org/pypi"
	pythonMaxPackages       = 8
	pythonMaxWheelBytes     = 10 << 20 // 10 MiB per wheel
	pythonMaxExtractedBytes = 64 << 20 // 64 MiB extracted per call, all packages
	pythonPyPITimeout       = 15 * time.Second
)

var (
	rePkgName = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9_.-]{0,127}[A-Za-z0-9])?$`)
	rePkgVer  = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9_.!+-]{0,63}[A-Za-z0-9])?$`)
)

type pypiFile struct {
	Filename    string `json:"filename"`
	Packagetype string `json:"packagetype"`
	URL         string `json:"url"`
	Size        int64  `json:"size"`
	Digests     struct {
		SHA256 string `json:"sha256"`
	} `json:"digests"`
}

type pypiRelease struct {
	Info struct {
		Version string `json:"version"`
	} `json:"info"`
	URLs []pypiFile `json:"urls"`
}

// preparedPackage is a verified wheel ready to extract into the sandbox.
type preparedPackage struct {
	key  string // safe dir name, e.g. "openpyxl-3.1.5"
	data []byte // raw .whl bytes
}

// parsePackageSpec splits "name" or "name==version", rejecting everything else.
func parsePackageSpec(spec string) (name, version string, err error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", "", errors.New("empty package spec")
	}
	name, version, _ = strings.Cut(spec, "==")
	if strings.Contains(name, "=") || strings.Contains(version, "=") {
		return "", "", fmt.Errorf("invalid package spec %q: only \"name\" or \"name==version\" allowed", spec)
	}
	name = strings.TrimSpace(name)
	version = strings.TrimSpace(version)
	if !rePkgName.MatchString(name) {
		return "", "", fmt.Errorf("invalid package name %q", name)
	}
	if version != "" && !rePkgVer.MatchString(version) {
		return "", "", fmt.Errorf("invalid version %q in spec %q", version, spec)
	}
	return name, version, nil
}

func pypiGET(ctx context.Context, url string, maxBytes int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ai-ui-python-sandbox/1.0")
	client := &http.Client{Timeout: pythonPyPITimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pypi request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pypi request failed: %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading pypi response: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("pypi response exceeds %d bytes", maxBytes)
	}
	return data, nil
}

func fetchPyPIRelease(ctx context.Context, name, version string) (*pypiRelease, error) {
	var url string
	if version == "" {
		url = fmt.Sprintf("%s/%s/json", pythonPyPIBase, name)
	} else {
		url = fmt.Sprintf("%s/%s/%s/json", pythonPyPIBase, name, version)
	}
	data, err := pypiGET(ctx, url, 1<<20)
	if err != nil {
		return nil, err
	}
	var rel pypiRelease
	if err := json.Unmarshal(data, &rel); err != nil {
		return nil, fmt.Errorf("parsing pypi response: %w", err)
	}
	if version == "" {
		version = rel.Info.Version
	}
	if version == "" || len(rel.URLs) == 0 {
		return nil, fmt.Errorf("package %q has no release files", name)
	}
	return &rel, nil
}

// selectPureWheel picks a pure-Python wheel (platform-independent by
// filename convention: *-none-any.whl). C-extension wheels always carry a
// platform tag (win/macosx/manylinux/musllinux) and never match.
func selectPureWheel(files []pypiFile) (*pypiFile, error) {
	var fallback *pypiFile
	for i := range files {
		f := &files[i]
		if f.Packagetype != "bdist_wheel" {
			continue
		}
		fn := f.Filename
		if !strings.HasSuffix(fn, ".whl") || !strings.HasSuffix(fn, "-none-any.whl") {
			continue
		}
		if strings.Contains(fn, "-py3-none-any.whl") {
			return f, nil
		}
		if fallback == nil {
			fallback = f
		}
	}
	if fallback != nil {
		return fallback, nil
	}
	return nil, errors.New("no pure-Python wheel found (this package needs C extensions, unsupported in the sandbox)")
}

func fetchWheelBytes(ctx context.Context, cacheDir string, f *pypiFile) ([]byte, error) {
	safe := filepath.Base(f.Filename)
	if safe != f.Filename || strings.ContainsAny(safe, `/\`) {
		return nil, fmt.Errorf("unsafe wheel filename %q", f.Filename)
	}
	if f.Size > pythonMaxWheelBytes {
		return nil, fmt.Errorf("wheel %q exceeds %d bytes", safe, pythonMaxWheelBytes)
	}
	want := strings.ToLower(f.Digests.SHA256)
	if want == "" {
		return nil, fmt.Errorf("pypi gave no sha256 for %q, refusing", safe)
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}
	if b, err := os.ReadFile(filepath.Join(cacheDir, safe)); err == nil {
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) == want {
			return b, nil
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ai-ui-python-sandbox/1.0")
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading wheel: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading wheel: %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, pythonMaxWheelBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading wheel: %w", err)
	}
	if int64(len(data)) > pythonMaxWheelBytes {
		return nil, fmt.Errorf("wheel exceeds %d bytes", pythonMaxWheelBytes)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != want {
		return nil, errors.New("wheel failed integrity check (sha256 mismatch vs pypi)")
	}
	_ = os.WriteFile(filepath.Join(cacheDir, safe), data, 0o644) // cache best-effort
	return data, nil
}

// preparePythonPackages resolves specs to verified wheel bytes (cached).
func preparePythonPackages(ctx context.Context, specs []string) ([]preparedPackage, error) {
	if len(specs) > pythonMaxPackages {
		return nil, fmt.Errorf("at most %d packages per call (%d given)", pythonMaxPackages, len(specs))
	}
	seen := make(map[string]struct{}, len(specs))
	out := make([]preparedPackage, 0, len(specs))
	cacheDir := filepath.Join(pythonEngineDir(), "wheels")
	for _, spec := range specs {
		name, version, err := parsePackageSpec(spec)
		if err != nil {
			return nil, err
		}
		rel, err := fetchPyPIRelease(ctx, name, version)
		if err != nil {
			return nil, fmt.Errorf("package %q: %w", spec, err)
		}
		if version == "" {
			version = rel.Info.Version
		}
		key := strings.ToLower(name) + "-" + version
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		wheel, err := selectPureWheel(rel.URLs)
		if err != nil {
			return nil, fmt.Errorf("package %q: %w", spec, err)
		}
		data, err := fetchWheelBytes(ctx, cacheDir, wheel)
		if err != nil {
			return nil, fmt.Errorf("package %q: %w", spec, err)
		}
		out = append(out, preparedPackage{key: key, data: data})
	}
	return out, nil
}

// extractWheelIntoSandbox unpacks wheel bytes under <root>/packages/<key>/.
// Zip-slip guarded; native binaries (.so/.pyd/.dll) skipped — the WASI guest
// cannot load them anyway. Returns the guest PYTHONPATH entry.
func extractWheelIntoSandbox(root string, pkg preparedPackage, budget *int64) (string, error) {
	key := filepath.Base(pkg.key)
	if key != pkg.key || strings.ContainsAny(key, `/\`) || key == "" || key == "." {
		return "", fmt.Errorf("unsafe package key %q", pkg.key)
	}
	dest := filepath.Join(root, "packages", key)
	zr, err := zip.NewReader(bytes.NewReader(pkg.data), int64(len(pkg.data)))
	if err != nil {
		return "", fmt.Errorf("invalid wheel for %q: %w", key, err)
	}
	for _, f := range zr.File {
		name := filepath.ToSlash(f.Name)
		if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, "..") {
			return "", fmt.Errorf("wheel %q contains unsafe path %q", key, f.Name)
		}
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".so") || strings.HasSuffix(lower, ".pyd") || strings.HasSuffix(lower, ".dll") {
			continue // native code: unloadable in WASI, skip to save space
		}
		if strings.HasSuffix(name, "/") {
			continue
		}
		if f.UncompressedSize64 > uint64(*budget) {
			return "", fmt.Errorf("package extraction exceeds budget (%d bytes)", pythonMaxExtractedBytes)
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		content, err := io.ReadAll(io.LimitReader(rc, *budget+1))
		rc.Close()
		if err != nil {
			return "", err
		}
		*budget -= int64(len(content))
		if *budget < 0 {
			return "", fmt.Errorf("package extraction exceeds budget (%d bytes)", pythonMaxExtractedBytes)
		}
		dst := filepath.Join(dest, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(dst, content, 0o644); err != nil {
			return "", err
		}
	}
	return "/packages/" + key, nil
}
