package tools

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strings"
	"testing"

	"github.com/Bajahaw/ai-ui/cmd/data"
	"github.com/Bajahaw/ai-ui/cmd/secrets"
	_ "modernc.org/sqlite"
)

func TestValidateHostname_RejectsIPsAndLocal(t *testing.T) {
	cases := []struct {
		host    string
		wantErr bool
	}{
		{"example.com", false},
		{"api.github.com", false},
		{"127.0.0.1", true},
		{"::1", true},
		{"0x7f000001", true},
		{"2130706433", true},
		{"0177.0.0.1", true},
		{"localhost", true},
		{"foo.localhost", true},
		{"foo.local", true},
		{"foo.internal", true},
		{"metadata.google.internal", true},
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"[::1]", true},
		{"", true},
		{"bad..host", true},
		{"-bad.com", true},
	}
	for _, tc := range cases {
		err := validateHostname(tc.host)
		if tc.wantErr && err == nil {
			t.Errorf("validateHostname(%q) = nil, want error", tc.host)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("validateHostname(%q) = %v, want nil", tc.host, err)
		}
	}
}

func TestAssertPublicIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1",
		"127.0.0.2",
		"0.0.0.0",
		"10.1.2.3",
		"172.16.0.1",
		"172.31.255.255",
		"192.168.0.1",
		"169.254.169.254",
		"100.64.0.1",
		"100.127.0.1",
		"192.0.2.1",
		"198.51.100.1",
		"203.0.113.1",
		"198.18.0.1",
		"224.0.0.1",
		"::1",
		"fe80::1",
		"fc00::1",
		"fd12:3456:789a::1",
		"2001:db8::1",
		"ff02::1",
	}
	for _, s := range blocked {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("ParseIP(%q) failed", s)
		}
		if err := assertPublicIP(ip); err == nil {
			t.Errorf("assertPublicIP(%s) = nil, want error", s)
		}
	}

	allowed := []string{
		"1.1.1.1",
		"8.8.8.8",
		"20.0.0.1",
		"2001:4860:4860::8888",
	}
	for _, s := range allowed {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("ParseIP(%q) failed", s)
		}
		if err := assertPublicIP(ip); err != nil {
			t.Errorf("assertPublicIP(%s) = %v, want nil", s, err)
		}
	}
}

func TestValidatePublicHTTPSURL_SchemeAndIP(t *testing.T) {
	// http rejected
	if _, err := validatePublicHTTPSURL("http://example.com/"); err == nil {
		t.Fatal("expected http scheme rejection")
	}
	// IP host rejected (no DNS)
	if _, err := validatePublicHTTPSURL("https://1.1.1.1/"); err == nil {
		t.Fatal("expected IP host rejection")
	}
	if _, err := validatePublicHTTPSURL("https://127.0.0.1/"); err == nil {
		t.Fatal("expected loopback IP rejection")
	}
	// userinfo rejected
	if _, err := validatePublicHTTPSURL("https://user:pass@example.com/"); err == nil {
		t.Fatal("expected userinfo rejection")
	}
	// whitespace
	if _, err := validatePublicHTTPSURL("https://example.com/path with space"); err == nil {
		t.Fatal("expected whitespace rejection")
	}
}

func TestValidatePublicHTTPSURL_PublicHost(t *testing.T) {
	// example.com is reserved documentation TLD but resolves publicly in practice
	// via IANA — use a host we control via mock? Real DNS for example.com is fine for CI.
	u, err := validatePublicHTTPSURL("https://example.com/path?q=1")
	if err != nil {
		t.Fatalf("example.com should be allowed: %v", err)
	}
	if u.Scheme != "https" || u.Hostname() != "example.com" {
		t.Fatalf("unexpected url: %v", u)
	}
}

func TestApplySafeHeaders(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	err := applySafeHeaders(req, map[string]string{
		"Authorization": "Bearer tok",
		"Content-Type":  "application/json",
		"X-Custom":      "yes",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer tok" {
		t.Fatalf("Authorization = %q", got)
	}

	req2, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err := applySafeHeaders(req2, map[string]string{"Host": "evil.com"}); err == nil {
		t.Fatal("expected Host header blocked")
	}
	if err := applySafeHeaders(req2, map[string]string{"Transfer-Encoding": "chunked"}); err == nil {
		t.Fatal("expected TE blocked")
	}
	if err := applySafeHeaders(req2, map[string]string{"X-Test": "a\r\nInjected: 1"}); err == nil {
		t.Fatal("expected CR/LF blocked")
	}
}

func TestDoSafeHTTPRequest_HappyPath(t *testing.T) {
	// TLS server with public-looking hostname is hard without DNS hijack.
	// Instead unit-test client against httptest by temporarily swapping dial is heavy.
	// Validate end-to-end policy via a local HTTPS server is still blocked (IP/localhost).
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "secret-internal")
	}))
	defer srv.Close()

	// httptest URL is https://127.0.0.1:port — must be rejected
	_, err := doSafeHTTPRequest(httpRequestParams{
		URL:    srv.URL + "/x",
		Method: "GET",
	})
	if err == nil {
		t.Fatal("expected local httptest URL to be rejected")
	}
	if !strings.Contains(err.Error(), "IP") && !strings.Contains(err.Error(), "loopback") && !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDoSafeHTTPRequest_MethodAndBody(t *testing.T) {
	_, err := doSafeHTTPRequest(httpRequestParams{
		URL:    "https://example.com/",
		Method: "TRACE",
	})
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("TRACE should be rejected: %v", err)
	}

	big := strings.Repeat("a", httpRequestMaxBody+1)
	_, err = doSafeHTTPRequest(httpRequestParams{
		URL:    "https://example.com/",
		Method: "POST",
		Body:   httpRequestBody{Raw: big, Set: true},
	})
	if err == nil || !strings.Contains(err.Error(), "body exceeds") {
		t.Fatalf("oversized body should be rejected: %v", err)
	}
}

func TestHTTPRequestBody_Unmarshal(t *testing.T) {
	// String body stays raw
	var p1 httpRequestParams
	if err := json.Unmarshal([]byte(`{"url":"https://example.com","body":"plain text"}`), &p1); err != nil {
		t.Fatal(err)
	}
	if p1.Body.Raw != "plain text" || p1.Body.FromJSON {
		t.Fatalf("string body: %+v", p1.Body)
	}

	// Object body marshaled to JSON
	var p2 httpRequestParams
	if err := json.Unmarshal([]byte(`{"url":"https://example.com","body":{"name":"Alice","n":1}}`), &p2); err != nil {
		t.Fatal(err)
	}
	if !p2.Body.FromJSON {
		t.Fatal("expected FromJSON")
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(p2.Body.Raw), &got); err != nil {
		t.Fatal(err)
	}
	if got["name"] != "Alice" || got["n"].(float64) != 1 {
		t.Fatalf("object body: %s", p2.Body.Raw)
	}

	// Array body
	var p3 httpRequestParams
	if err := json.Unmarshal([]byte(`{"url":"https://example.com","body":[1,2]}`), &p3); err != nil {
		t.Fatal(err)
	}
	if !p3.Body.FromJSON || p3.Body.Raw != "[1,2]" {
		t.Fatalf("array body: %+v", p3.Body)
	}

	// Already-serialized JSON string must not be double-encoded
	var p4 httpRequestParams
	if err := json.Unmarshal([]byte(`{"url":"https://example.com","body":"{\"name\":\"Bob\"}"}`), &p4); err != nil {
		t.Fatal(err)
	}
	if p4.Body.FromJSON || p4.Body.Raw != `{"name":"Bob"}` {
		t.Fatalf("json string body: %+v", p4.Body)
	}

	// Omitted body
	var p5 httpRequestParams
	if err := json.Unmarshal([]byte(`{"url":"https://example.com"}`), &p5); err != nil {
		t.Fatal(err)
	}
	if p5.Body.Set || p5.Body.Raw != "" {
		t.Fatalf("omitted body: %+v", p5.Body)
	}
}

func TestCheckRedirect_BlocksPrivateTarget(t *testing.T) {
	client := newSafeHTTPClient()
	// Simulate redirect check directly
	req, _ := http.NewRequest(http.MethodGet, "https://127.0.0.1/", nil)
	err := client.CheckRedirect(req, []*http.Request{req})
	if err == nil {
		t.Fatal("redirect to loopback IP should fail")
	}
}

func TestHTTPRequestTool_ParseError(t *testing.T) {
	out := httpRequestTool(`{not-json`, "user")
	if !strings.Contains(out.Content, "error decoding") {
		t.Fatalf("got %q", out.Content)
	}
}

func TestHTTPRequestTool_BlocksSSRFVectors(t *testing.T) {
	vectors := []string{
		`{"url":"http://example.com"}`,
		`{"url":"https://127.0.0.1/"}`,
		`{"url":"https://localhost/"}`,
		`{"url":"https://[::1]/"}`,
		`{"url":"https://169.254.169.254/latest/meta-data/"}`,
		`{"url":"https://metadata.google.internal/"}`,
		`{"url":"file:///etc/passwd"}`,
		`{"url":"https://user:pass@example.com/"}`,
	}
	for _, args := range vectors {
		out := httpRequestTool(args, "user")
		if !strings.Contains(out.Content, "failed") && !strings.Contains(out.Content, "error") {
			t.Errorf("vector %s got success-like output: %s", args, out.Content)
		}
		if strings.Contains(out.Content, "Status: 200") {
			t.Errorf("vector %s must not succeed: %s", args, out.Content)
		}
	}
}

func setupSecretsDB(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	dbPath := path.Join(dir, "test.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := data.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO Users (username, pass_hash) VALUES ('alice', 'x')`); err != nil {
		t.Fatal(err)
	}
	secrets.SetupSecrets(nil, db)
	if _, err := secrets.Create("alice", "API_KEY", "zone_abc123"); err != nil {
		t.Fatal(err)
	}
	if _, err := secrets.Create("alice", "TOKEN", "tok_xyz"); err != nil {
		t.Fatal(err)
	}
	if _, err := secrets.Create("alice", "ODD", "a/b c?d=e"); err != nil {
		t.Fatal(err)
	}
}

func TestInjectHTTPRequestSecrets(t *testing.T) {
	// Unit-test expansion rules without DB: empty map => unknown secret errors.
	params := &httpRequestParams{
		URL:     "https://api.example.com/v1?token=$secrets.API_KEY$",
		Headers: map[string]string{"Authorization": "Bearer $secrets.API_KEY$", "Cookie": "sid=$secrets.API_KEY$"},
		Body:    httpRequestBody{Raw: `{"ok":true}`, Set: true},
	}
	err := injectHTTPRequestSecrets(params, "nobody")
	if err == nil {
		t.Fatal("expected unknown secret without store")
	}

	// Body rejected
	params2 := &httpRequestParams{
		URL:  "https://api.example.com/",
		Body: httpRequestBody{Raw: `$secrets.API_KEY$`, Set: true},
	}
	if err := injectHTTPRequestSecrets(params2, "nobody"); err == nil || !strings.Contains(err.Error(), "body") {
		t.Fatalf("expected body rejection, got %v", err)
	}

	// Path allowed (unknown secret without store)
	params3 := &httpRequestParams{
		URL: "https://api.example.com/$secrets.API_KEY$/x",
	}
	if err := injectHTTPRequestSecrets(params3, "nobody"); err == nil || !strings.Contains(err.Error(), "unknown secret") {
		t.Fatalf("expected unknown secret on path expand, got %v", err)
	}

	// Host still rejected
	params4 := &httpRequestParams{
		URL: "https://$secrets.API_KEY$.example.com/",
	}
	if err := injectHTTPRequestSecrets(params4, "nobody"); err == nil || !strings.Contains(err.Error(), "host") {
		t.Fatalf("expected host rejection, got %v", err)
	}
}

func TestInjectHTTPRequestSecrets_ExpandsPathQueryHeaders(t *testing.T) {
	setupSecretsDB(t)

	// Cloudflare-style path secret
	p := &httpRequestParams{
		URL: "https://api.cloudflare.com/client/v4/zones/$secrets.API_KEY$/purge_cache",
		Headers: map[string]string{
			"Authorization": "Bearer $secrets.TOKEN$",
		},
	}
	if err := injectHTTPRequestSecrets(p, "alice"); err != nil {
		t.Fatal(err)
	}
	if p.URL != "https://api.cloudflare.com/client/v4/zones/zone_abc123/purge_cache" {
		t.Fatalf("path expand: got %q", p.URL)
	}
	if p.Headers["Authorization"] != "Bearer tok_xyz" {
		t.Fatalf("header expand: got %q", p.Headers["Authorization"])
	}
	// Placeholder must not remain
	if strings.Contains(p.URL, "$secrets.") || strings.Contains(p.Headers["Authorization"], "$secrets.") {
		t.Fatal("placeholder leaked after expand")
	}

	// Query only
	q := &httpRequestParams{
		URL: "https://api.example.com/v1?token=$secrets.TOKEN$&ok=1",
	}
	if err := injectHTTPRequestSecrets(q, "alice"); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(q.URL)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("token") != "tok_xyz" || u.Query().Get("ok") != "1" {
		t.Fatalf("query expand: %q", q.URL)
	}

	// Path + query together
	both := &httpRequestParams{
		URL: "https://api.example.com/zones/$secrets.API_KEY$/items?key=$secrets.TOKEN$",
	}
	if err := injectHTTPRequestSecrets(both, "alice"); err != nil {
		t.Fatal(err)
	}
	wantBoth := "https://api.example.com/zones/zone_abc123/items?key=tok_xyz"
	if both.URL != wantBoth {
		t.Fatalf("path+query: got %q want %q", both.URL, wantBoth)
	}

	// Secret with reserved chars is expanded (no placeholder left); host unchanged
	odd := &httpRequestParams{
		URL: "https://api.example.com/p/$secrets.ODD$/tail",
	}
	if err := injectHTTPRequestSecrets(odd, "alice"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(odd.URL, "$secrets.") {
		t.Fatalf("placeholder remained: %q", odd.URL)
	}
	ou, err := url.Parse(odd.URL)
	if err != nil {
		t.Fatal(err)
	}
	if ou.Host != "api.example.com" {
		t.Fatalf("host changed: %q", ou.Host)
	}
	if !strings.Contains(odd.URL, "a") || !strings.Contains(ou.Path, "a") {
		t.Fatalf("expanded value missing from url: %q path=%q", odd.URL, ou.Path)
	}

	// Host / userinfo / body still rejected with real secrets present
	for _, tc := range []struct {
		name string
		p    *httpRequestParams
		sub  string
	}{
		{"host", &httpRequestParams{URL: "https://$secrets.API_KEY$.example.com/"}, "host"},
		{"userinfo", &httpRequestParams{URL: "https://$secrets.TOKEN$@api.example.com/"}, "userinfo"},
		{"body", &httpRequestParams{URL: "https://api.example.com/", Body: httpRequestBody{Raw: `$secrets.TOKEN$`, Set: true}}, "body"},
		{"header name", &httpRequestParams{
			URL:     "https://api.example.com/",
			Headers: map[string]string{"X-$secrets.TOKEN$": "v"},
		}, "header names"},
	} {
		err := injectHTTPRequestSecrets(tc.p, "alice")
		if err == nil || !strings.Contains(err.Error(), tc.sub) {
			t.Fatalf("%s: expected error containing %q, got %v", tc.name, tc.sub, err)
		}
	}

	// Other user cannot use alice secrets
	iso := &httpRequestParams{
		URL: "https://api.example.com/$secrets.API_KEY$/x",
	}
	if err := injectHTTPRequestSecrets(iso, "bob"); err == nil || !strings.Contains(err.Error(), "unknown secret") {
		t.Fatalf("isolation: got %v", err)
	}
}

// Ensure redirect validation uses full URL string including host.
func TestValidatePublicHTTPSURL_FragmentsDropped(t *testing.T) {
	u, err := validatePublicHTTPSURL("https://example.com/a#frag")
	if err != nil {
		t.Fatal(err)
	}
	if u.Fragment != "" {
		t.Fatalf("fragment should be stripped, got %q", u.Fragment)
	}
}

func TestIsObfuscatedIPHost(t *testing.T) {
	if !isObfuscatedIPHost("2130706433") {
		t.Fatal("expected decimal IP host")
	}
	if !isObfuscatedIPHost("0x7f000001") {
		t.Fatal("expected hex IP host")
	}
	if !isObfuscatedIPHost("0177.0.0.1") {
		t.Fatal("expected dotted numeric")
	}
	if isObfuscatedIPHost("example.com") {
		t.Fatal("domain is not obfuscated IP")
	}
}

func TestFormatResponseBody_TextAndBinary(t *testing.T) {
	opts := httpBodyFormatOpts{}
	textOut := formatResponseBody([]byte(`{"ok":true}`), "application/json", opts)
	if !strings.Contains(textOut, `{"ok":true}`) {
		t.Fatalf("json body should be included: %s", textOut)
	}

	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x01}
	binOut := formatResponseBody(png, "image/png", opts)
	if strings.Contains(binOut, string(png)) {
		t.Fatal("raw binary must not appear in output")
	}
	if !strings.Contains(binOut, "binary body omitted") {
		t.Fatalf("expected omit notice: %s", binOut)
	}
	if !strings.Contains(binOut, "image/png") {
		t.Fatalf("expected media type: %s", binOut)
	}

	// Large text truncated in output (default/reduced = 32 KiB)
	big := []byte(strings.Repeat("x", httpRequestMaxTextOut+100))
	bigOut := formatResponseBody(big, "text/plain", opts)
	if !strings.Contains(bigOut, "text output truncated") {
		t.Fatalf("expected text truncation note: %s", bigOut[len(bigOut)-80:])
	}
	if len(bigOut) > httpRequestMaxTextOut+200 {
		t.Fatalf("output too large: %d", len(bigOut))
	}

	// Verbose allows up to 256 KiB before truncating
	mid := []byte(strings.Repeat("y", httpRequestMaxTextOut+100))
	midOut := formatResponseBody(mid, "text/plain", httpBodyFormatOpts{Verbose: true})
	if strings.Contains(midOut, "text output truncated") {
		t.Fatalf("verbose should not truncate at reduced cap: %s", midOut[len(midOut)-80:])
	}
	huge := []byte(strings.Repeat("z", httpRequestMaxTextOutVerbose+100))
	hugeOut := formatResponseBody(huge, "text/plain", httpBodyFormatOpts{Verbose: true})
	if !strings.Contains(hugeOut, "text output truncated") {
		t.Fatalf("verbose should truncate at verbose cap: %s", hugeOut[len(hugeOut)-80:])
	}
	if len(hugeOut) > httpRequestMaxTextOutVerbose+200 {
		t.Fatalf("verbose output too large: %d", len(hugeOut))
	}
}

func testBase64Blob(n int) string {
	// Mixed alphabet + padding; length multiple of 4.
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var b strings.Builder
	b.Grow(n)
	for i := 0; i < n-2; i++ {
		b.WriteByte(alphabet[i%len(alphabet)])
	}
	b.WriteString("==")
	return b.String()
}

func TestFormatResponseBody_ReducesBase64AndHTML(t *testing.T) {
	opts := httpBodyFormatOpts{}
	b64 := testBase64Blob(300)
	html := `<html><head><style>.x{color:red}</style><script>alert(1)</script></head>` +
		`<body><p>hi</p><img src="data:image/png;base64,` + b64 + `"/></body></html>`
	out := formatResponseBody([]byte(html), "text/html", opts)
	if strings.Contains(out, "alert(1)") || strings.Contains(out, ".x{color") {
		t.Fatalf("script/style should be scrubbed: %s", out)
	}
	if strings.Contains(out, b64) {
		t.Fatalf("base64 payload should be omitted: %s", out[:min(200, len(out))])
	}
	if !strings.Contains(out, "hi") {
		t.Fatalf("visible text should remain: %s", out)
	}
	if !strings.Contains(out, "html scrub") && !strings.Contains(out, "omitted") {
		t.Fatalf("expected reduction notes: %s", out[max(0, len(out)-120):])
	}
}

func TestFormatResponseBody_ReducesJSON(t *testing.T) {
	opts := httpBodyFormatOpts{}
	b64 := testBase64Blob(300)
	longNote := strings.Repeat("n", 600)
	payload := map[string]any{
		"ok":    true,
		"blob":  b64,
		"note":  longNote,
		"items": []any{1, 2, 3, 4, 5},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	out := formatResponseBody(raw, "application/json", opts)
	if strings.Contains(out, b64) {
		t.Fatal("json base64 should be omitted")
	}
	if !strings.Contains(out, longNote) {
		t.Fatal("long non-base64 strings should be kept")
	}
	if !strings.Contains(out, `"ok":true`) {
		t.Fatalf("structure should remain: %s", out)
	}
	if !strings.Contains(out, `[1,2,3,4,5]`) {
		t.Fatalf("arrays should be kept: %s", out)
	}
}

func TestFormatResponseBody_VerboseKeepsPayload(t *testing.T) {
	b64 := testBase64Blob(300)
	html := `<html><body><script>keepme()</script><p>` + b64 + `</p></body></html>`
	out := formatResponseBody([]byte(html), "text/html", httpBodyFormatOpts{Verbose: true})
	if !strings.Contains(out, "keepme()") {
		t.Fatalf("verbose should keep script: %s", out)
	}
	if !strings.Contains(out, b64) {
		t.Fatalf("verbose should keep base64: %s", out[:min(200, len(out))])
	}
}

func TestReadLimitedBody(t *testing.T) {
	r := strings.NewReader(strings.Repeat("a", 100))
	data, trunc, err := readLimitedBody(r, 50)
	if err != nil {
		t.Fatal(err)
	}
	if !trunc || len(data) != 50 {
		t.Fatalf("trunc=%v len=%d", trunc, len(data))
	}

	r2 := strings.NewReader("hi")
	data, trunc, err = readLimitedBody(r2, 50)
	if err != nil || trunc || string(data) != "hi" {
		t.Fatalf("got %q trunc=%v err=%v", data, trunc, err)
	}
}

func TestIsTextualMediaType(t *testing.T) {
	if !isTextualMediaType("application/json") || !isTextualMediaType("text/html") {
		t.Fatal("expected textual")
	}
	if isTextualMediaType("image/png") || isTextualMediaType("application/octet-stream") {
		t.Fatal("expected non-textual")
	}
	if !isTextualMediaType("application/vnd.foo+json") {
		t.Fatal("expected +json textual")
	}
}

func TestLooksLikeUTF8Text(t *testing.T) {
	if !looksLikeUTF8Text([]byte("hello world\n")) {
		t.Fatal("plain text")
	}
	if looksLikeUTF8Text([]byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0x00, 0x00}) {
		t.Fatal("binary should not look like text")
	}
}

func TestValidateJSONPointers(t *testing.T) {
	if err := validateJSONPointers(nil); err != nil {
		t.Fatal(err)
	}
	if err := validateJSONPointers([]string{"/a", "/b/0"}); err != nil {
		t.Fatal(err)
	}
	if err := validateJSONPointers([]string{""}); err != nil {
		t.Fatal(err)
	}
	if err := validateJSONPointers([]string{"a"}); err == nil {
		t.Fatal("expected missing leading slash")
	}
	if err := validateJSONPointers([]string{"/a", "/a"}); err == nil {
		t.Fatal("expected duplicate rejection")
	}
	tooMany := make([]string, httpRequestMaxJSONPointers+1)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("/%d", i)
	}
	if err := validateJSONPointers(tooMany); err == nil {
		t.Fatal("expected too many pointers")
	}
	long := "/" + strings.Repeat("x", httpRequestMaxJSONPointerLen)
	if err := validateJSONPointers([]string{long}); err == nil {
		t.Fatal("expected pointer too long")
	}
}

func TestEvalJSONPointer(t *testing.T) {
	var root any
	raw := []byte(`{"a":{"b":[10,{"c":"x","d/e":1,"t~t":2}]},"n":null}`)
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}

	v, err := evalJSONPointer(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(map[string]any); !ok {
		t.Fatalf("root want object, got %T", v)
	}

	v, err = evalJSONPointer(root, "/a/b/0")
	if err != nil {
		t.Fatal(err)
	}
	if v.(float64) != 10 {
		t.Fatalf("got %v", v)
	}

	v, err = evalJSONPointer(root, "/a/b/1/c")
	if err != nil {
		t.Fatal(err)
	}
	if v != "x" {
		t.Fatalf("got %v", v)
	}

	v, err = evalJSONPointer(root, "/a/b/1/d~1e")
	if err != nil {
		t.Fatal(err)
	}
	if v.(float64) != 1 {
		t.Fatalf("escaped slash key: %v", v)
	}

	v, err = evalJSONPointer(root, "/a/b/1/t~0t")
	if err != nil {
		t.Fatal(err)
	}
	if v.(float64) != 2 {
		t.Fatalf("escaped tilde key: %v", v)
	}

	if _, err := evalJSONPointer(root, "/missing"); err == nil {
		t.Fatal("expected missing key")
	}
	if _, err := evalJSONPointer(root, "/a/b/99"); err == nil {
		t.Fatal("expected OOB index")
	}
	if _, err := evalJSONPointer(root, "/a/b/1/c/extra"); err == nil {
		t.Fatal("expected traverse into string fail")
	}
}

func TestFilterJSONBodyByPointers(t *testing.T) {
	body := []byte(`{"ok":true,"data":{"items":[{"id":1},{"id":2}]},"meta":{"total":2}}`)
	out, err := filterJSONBodyByPointers(body, []string{"/data/items/0/id", "/meta/total"})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got["/data/items/0/id"].(float64) != 1 {
		t.Fatalf("id: %v", got["/data/items/0/id"])
	}
	if got["/meta/total"].(float64) != 2 {
		t.Fatalf("total: %v", got["/meta/total"])
	}

	if _, err := filterJSONBodyByPointers([]byte("not-json"), []string{"/a"}); err == nil {
		t.Fatal("expected non-JSON error")
	}
	if _, err := filterJSONBodyByPointers(body, []string{"/nope"}); err == nil {
		t.Fatal("expected missing path error")
	}
}

func TestFormatHTTPResponse_JSONPointers(t *testing.T) {
	resp := &http.Response{
		StatusCode:    200,
		Status:        "200 OK",
		ContentLength: 100,
		Header:        http.Header{"Content-Type": []string{"application/json"}},
		Request:       httptest.NewRequest(http.MethodGet, "https://example.com/api", nil),
	}
	filtered := []byte(`{"/a":1}`)
	out := formatHTTPResponse(resp, filtered, false, httpBodyFormatOpts{
		JSONPointers: []string{"/a"},
		WireBytes:    100,
	})
	if !strings.Contains(out, "JSON-Pointers: /a\n") {
		t.Fatalf("missing pointers line: %s", out)
	}
	if !strings.Contains(out, "Bytes-Read: 100\n") {
		t.Fatalf("want wire bytes: %s", out)
	}
	if !strings.Contains(out, `{"/a":1}`) {
		t.Fatalf("want filtered body: %s", out)
	}
}

func TestDoSafeHTTPRequest_JSONPointersValidatedEarly(t *testing.T) {
	_, err := doSafeHTTPRequest(httpRequestParams{
		URL:          "https://example.com/",
		JSONPointers: []string{"no-slash"},
	})
	if err == nil || !strings.Contains(err.Error(), "json_pointer") {
		t.Fatalf("expected pointer validation error, got %v", err)
	}
}

func TestValidateCSSSelectors(t *testing.T) {
	if err := validateCSSSelectors(nil); err != nil {
		t.Fatal(err)
	}
	if err := validateCSSSelectors([]string{"a.result-link", ".result__snippet"}); err != nil {
		t.Fatal(err)
	}
	if err := validateCSSSelectors([]string{""}); err == nil {
		t.Fatal("expected empty rejection")
	}
	if err := validateCSSSelectors([]string{"a", "a"}); err == nil {
		t.Fatal("expected duplicate rejection")
	}
	if err := validateCSSSelectors([]string{"a[href="}); err == nil {
		t.Fatal("expected invalid selector rejection")
	}
	tooMany := make([]string, httpRequestMaxCSSSelectors+1)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf(".c%d", i)
	}
	if err := validateCSSSelectors(tooMany); err == nil {
		t.Fatal("expected too many")
	}
}

func TestFilterHTMLBodyBySelectors(t *testing.T) {
	html := []byte(`<!doctype html><html><body>
		<div class="result"><a class="result-link" href="https://example.com/a">Alpha Title</a><span class="snippet">Alpha blurb</span></div>
		<div class="result"><a class="result-link" href="https://example.com/b">Beta Title</a><span class="snippet">Beta blurb</span></div>
		<script>evil()</script>
	</body></html>`)

	out, err := filterHTMLBodyBySelectors(html, []string{"a.result-link", ".snippet", ".missing"})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	links, ok := got["a.result-link"].([]any)
	if !ok || len(links) != 2 {
		t.Fatalf("links: %#v", got["a.result-link"])
	}
	first := links[0].(map[string]any)
	if first["text"] != "Alpha Title" || first["href"] != "https://example.com/a" {
		t.Fatalf("first link: %#v", first)
	}
	snips := got[".snippet"].([]any)
	if len(snips) != 2 {
		t.Fatalf("snippets: %#v", snips)
	}
	missing := got[".missing"].([]any)
	if len(missing) != 0 {
		t.Fatalf("missing should be empty: %#v", missing)
	}
}

func TestFormatHTTPResponse_CSSSelectors(t *testing.T) {
	resp := &http.Response{
		StatusCode:    200,
		Status:        "200 OK",
		ContentLength: 5000,
		Header:        http.Header{"Content-Type": []string{"text/html"}},
		Request:       httptest.NewRequest(http.MethodGet, "https://example.com/search?q=go", nil),
	}
	filtered := []byte(`{"a.main-link":[{"text":"Go","href":"https://go.dev"}]}`)
	out := formatHTTPResponse(resp, filtered, false, httpBodyFormatOpts{
		CSSSelectors: []string{"a.main-link"},
		WireBytes:    5000,
	})
	if !strings.Contains(out, "CSS-Selectors: a.main-link\n") {
		t.Fatalf("missing selectors line: %s", out)
	}
	if !strings.Contains(out, "Bytes-Read: 5000\n") {
		t.Fatalf("want wire bytes: %s", out)
	}
	if !strings.Contains(out, `"href":"https://go.dev"`) {
		t.Fatalf("want filtered body: %s", out)
	}
}

func TestDoSafeHTTPRequest_CSSAndJSONPointersExclusive(t *testing.T) {
	_, err := doSafeHTTPRequest(httpRequestParams{
		URL:          "https://example.com/",
		JSONPointers: []string{"/a"},
		CSSSelectors: []string{"a"},
	})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected mutual exclusion error, got %v", err)
	}
}

func TestFormatHTTPResponse_BinaryOmitsBody(t *testing.T) {
	resp := &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Header:     http.Header{"Content-Type": []string{"application/pdf"}},
		Request:    httptest.NewRequest(http.MethodGet, "https://example.com/file.pdf", nil),
	}
	body := []byte("%PDF-1.4 binary stuff \x00\x01\x02")
	out := formatHTTPResponse(resp, body, false, httpBodyFormatOpts{})
	if strings.Contains(out, "%PDF") && strings.Contains(out, "\x00") {
		// PDF magic may appear in hex prefix only — ensure raw NUL body not dumped as text dump after Body:
		idx := strings.Index(out, "Body:\n")
		if idx < 0 {
			t.Fatal("missing Body section")
		}
		bodySection := out[idx:]
		if strings.Contains(bodySection, "\x00") {
			t.Fatal("NUL bytes must not appear in body section")
		}
	}
	if !strings.Contains(out, "binary body omitted") {
		t.Fatalf("expected binary omit: %s", out)
	}
	if !strings.Contains(out, "Body-SHA256:") {
		t.Fatal("expected sha256")
	}
	if !strings.Contains(out, "Body-Mode: reduced") {
		t.Fatal("expected reduced body mode")
	}
}

// compile-time sanity: url used in error paths
var _ = fmt.Sprintf
var _ = url.URL{}
