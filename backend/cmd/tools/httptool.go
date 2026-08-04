package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Bajahaw/ai-ui/cmd/providers"
	"github.com/Bajahaw/ai-ui/cmd/secrets"
)

const (
	httpRequestTimeout      = 30 * time.Second
	httpRequestMaxRedirect  = 3
	httpRequestMaxBody      = 1 << 20 // 1 MiB request body
	httpRequestMaxRespRead  = 1 << 20 // 1 MiB max bytes read from response
	httpRequestMaxTextOut   = 256 << 10 // 256 KiB max text body in tool output
	httpRequestMaxURLLen    = 2048
	httpRequestMaxHeaderSz  = 8 << 10 // 8 KiB total custom headers
	httpRequestMaxHeaderOut = 4 << 10 // 4 KiB response headers in output
)

var allowedHTTPMethods = map[string]struct{}{
	http.MethodGet:    {},
	http.MethodHead:   {},
	http.MethodPost:   {},
	http.MethodPut:    {},
	http.MethodPatch:  {},
	http.MethodDelete: {},
}

// Hop-by-hop and request-smuggling sensitive headers must not be set by the model.
var blockedRequestHeaders = map[string]struct{}{
	"host":              {},
	"content-length":    {},
	"transfer-encoding": {},
	"connection":        {},
	"keep-alive":        {},
	"upgrade":           {},
	"te":                {},
	"trailer":           {},
	"proxy-connection":  {},
	"proxy-authenticate": {},
	"proxy-authorization": {},
	"accept-encoding":   {}, // we control decompression
}

// Hostnames that must never be contacted, regardless of DNS.
var blockedHostnames = map[string]struct{}{
	"localhost":                 {},
	"localhost.localdomain":     {},
	"metadata":                  {},
	"metadata.google.internal":  {},
	"metadata.goog":             {},
	"kubernetes":                {},
	"kubernetes.default":        {},
	"kubernetes.default.svc":    {},
	"kubernetes.default.svc.cluster.local": {},
}

type httpRequestParams struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

func httpRequestTool(args, user string) providers.ToolOutput {
	var params httpRequestParams
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error decoding arguments: %v", err)}
	}

	if err := injectHTTPRequestSecrets(&params, user); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("http_request failed: %v", err)}
	}

	result, err := doSafeHTTPRequest(params)
	if err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("http_request failed: %v", err)}
	}
	return providers.ToolOutput{Content: result}
}

// injectHTTPRequestSecrets expands $secrets.NAME$ only in header values and URL query.
// Body, path, host, and userinfo are rejected if they contain placeholders.
func injectHTTPRequestSecrets(params *httpRequestParams, user string) error {
	const marker = "$secrets."

	if strings.Contains(params.Body, marker) {
		return errors.New("secret placeholders are only allowed in headers and URL query, not body")
	}

	byName := secrets.GetValueMap(user)

	if params.Headers != nil {
		for k, v := range params.Headers {
			if strings.Contains(k, marker) {
				return errors.New("secret placeholders are not allowed in header names")
			}
			if !strings.Contains(v, marker) {
				continue
			}
			expanded, err := secrets.Expand(v, byName)
			if err != nil {
				return err
			}
			params.Headers[k] = expanded
		}
	}

	if !strings.Contains(params.URL, marker) {
		return nil
	}

	u, err := url.Parse(params.URL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	for _, part := range []string{u.Scheme, u.Host, u.Path, u.Fragment, u.Opaque} {
		if strings.Contains(part, marker) {
			return errors.New("secret placeholders are only allowed in headers and URL query, not path/host")
		}
	}
	if u.User != nil && strings.Contains(u.User.String(), marker) {
		return errors.New("secret placeholders are only allowed in headers and URL query, not userinfo")
	}
	if !strings.Contains(u.RawQuery, marker) {
		// Placeholder somewhere odd (e.g. malformed); refuse.
		return errors.New("secret placeholders are only allowed in headers and URL query")
	}
	expandedQuery, err := secrets.Expand(u.RawQuery, byName)
	if err != nil {
		return err
	}
	u.RawQuery = expandedQuery
	params.URL = u.String()
	return nil
}

func doSafeHTTPRequest(params httpRequestParams) (string, error) {
	method := strings.ToUpper(strings.TrimSpace(params.Method))
	if method == "" {
		method = http.MethodGet
	}
	if _, ok := allowedHTTPMethods[method]; !ok {
		return "", fmt.Errorf("method %q is not allowed", method)
	}

	if strings.TrimSpace(params.URL) == "" {
		return "", errors.New("url is required")
	}
	if len(params.URL) > httpRequestMaxURLLen {
		return "", fmt.Errorf("url exceeds maximum length of %d", httpRequestMaxURLLen)
	}

	parsed, err := validatePublicHTTPSURL(params.URL)
	if err != nil {
		return "", err
	}

	var bodyReader io.Reader
	reqBodyLen := 0
	if params.Body != "" && method != http.MethodGet && method != http.MethodHead {
		reqBodyLen = len(params.Body)
		if reqBodyLen > httpRequestMaxBody {
			return "", fmt.Errorf("request body exceeds maximum size of %d bytes (%d given)", httpRequestMaxBody, reqBodyLen)
		}
		bodyReader = strings.NewReader(params.Body)
	}

	ctx, cancel := context.WithTimeout(context.Background(), httpRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, parsed.String(), bodyReader)
	if err != nil {
		return "", fmt.Errorf("building request: %w", err)
	}
	if reqBodyLen > 0 {
		req.ContentLength = int64(reqBodyLen)
	}

	if err := applySafeHeaders(req, params.Headers); err != nil {
		return "", err
	}

	client := newSafeHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	// Never trust Content-Length alone; always cap bytes read from the wire.
	respBody, truncated, err := readLimitedBody(resp.Body, httpRequestMaxRespRead)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	return formatHTTPResponse(resp, respBody, truncated), nil
}

func readLimitedBody(r io.Reader, max int) (data []byte, truncated bool, err error) {
	limited := io.LimitReader(r, int64(max)+1)
	data, err = io.ReadAll(limited)
	if err != nil {
		return nil, false, err
	}
	if len(data) > max {
		data = data[:max]
		truncated = true
		// Drain a bit more so the peer is less likely to RST; ignore errors.
		_, _ = io.Copy(io.Discard, io.LimitReader(r, 64<<10))
	}
	return data, truncated, nil
}

func formatHTTPResponse(resp *http.Response, body []byte, truncated bool) string {
	ct := resp.Header.Get("Content-Type")
	mediaType := contentMediaType(ct)
	declaredLen := resp.ContentLength

	var b strings.Builder
	fmt.Fprintf(&b, "Status: %d %s\n", resp.StatusCode, http.StatusText(resp.StatusCode))
	if resp.Request != nil && resp.Request.URL != nil {
		fmt.Fprintf(&b, "Final-URL: %s\n", resp.Request.URL.String())
	}
	fmt.Fprintf(&b, "Content-Type: %s\n", emptyAs(ct, "(none)"))
	fmt.Fprintf(&b, "Bytes-Read: %d\n", len(body))
	if declaredLen >= 0 {
		fmt.Fprintf(&b, "Content-Length: %d\n", declaredLen)
	}
	if truncated {
		fmt.Fprintf(&b, "Truncated: true (read cap %d bytes)\n", httpRequestMaxRespRead)
	}

	b.WriteString("Response-Headers:\n")
	headerBytes := 0
	for k, vals := range resp.Header {
		if strings.EqualFold(k, "Set-Cookie") {
			line := fmt.Sprintf("  %s: (%d cookie(s) redacted)\n", k, len(vals))
			if headerBytes+len(line) > httpRequestMaxHeaderOut {
				b.WriteString("  ... (headers truncated)\n")
				break
			}
			b.WriteString(line)
			headerBytes += len(line)
			continue
		}
		for _, v := range vals {
			if len(v) > 512 {
				v = v[:512] + "…(truncated)"
			}
			line := fmt.Sprintf("  %s: %s\n", k, v)
			if headerBytes+len(line) > httpRequestMaxHeaderOut {
				b.WriteString("  ... (headers truncated)\n")
				headerBytes = httpRequestMaxHeaderOut
				break
			}
			b.WriteString(line)
			headerBytes += len(line)
		}
		if headerBytes >= httpRequestMaxHeaderOut {
			break
		}
	}

	sum := sha256.Sum256(body)
	fmt.Fprintf(&b, "Body-SHA256: %s\n", hex.EncodeToString(sum[:]))

	b.WriteString("Body:\n")
	b.WriteString(formatResponseBody(body, mediaType, truncated))
	return b.String()
}

func contentMediaType(ct string) string {
	ct = strings.TrimSpace(ct)
	if ct == "" {
		return ""
	}
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.ToLower(strings.TrimSpace(ct))
}

func emptyAs(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func formatResponseBody(body []byte, mediaType string, readTruncated bool) string {
	if len(body) == 0 {
		return "(empty)"
	}

	textual := isTextualMediaType(mediaType) || (mediaType == "" && looksLikeUTF8Text(body))
	if !textual {
		// Binary / unknown non-text: never dump raw bytes into the chat/model context.
		preview := 16
		if len(body) < preview {
			preview = len(body)
		}
		var b strings.Builder
		fmt.Fprintf(&b, "(binary body omitted)\nMedia-Type: %s\nSize-Bytes: %d\nHex-Prefix: %s",
			emptyAs(mediaType, "application/octet-stream"),
			len(body),
			hex.EncodeToString(body[:preview]),
		)
		if readTruncated {
			b.WriteString("\nNote: response was truncated by read cap; full remote size may be larger.")
		}
		return b.String()
	}

	text := body
	outTruncated := false
	if len(text) > httpRequestMaxTextOut {
		text = text[:httpRequestMaxTextOut]
		outTruncated = true
		// Avoid cutting mid-rune.
		for len(text) > 0 && !utf8.Valid(text) {
			text = text[:len(text)-1]
		}
	}

	// Replace NULs which break some UIs/DB paths.
	s := strings.ReplaceAll(string(text), "\x00", "\uFFFD")

	var b strings.Builder
	b.WriteString(s)
	if outTruncated || readTruncated {
		b.WriteString("\n\n[")
		if outTruncated {
			fmt.Fprintf(&b, "text output truncated to %d bytes", httpRequestMaxTextOut)
		}
		if readTruncated {
			if outTruncated {
				b.WriteString("; ")
			}
			fmt.Fprintf(&b, "wire read capped at %d bytes", httpRequestMaxRespRead)
		}
		b.WriteString("]")
	}
	return b.String()
}

func isTextualMediaType(mediaType string) bool {
	if mediaType == "" {
		return false
	}
	if strings.HasPrefix(mediaType, "text/") {
		return true
	}
	switch mediaType {
	case "application/json",
		"application/ld+json",
		"application/problem+json",
		"application/xml",
		"application/xhtml+xml",
		"application/javascript",
		"application/ecmascript",
		"application/x-www-form-urlencoded",
		"application/graphql",
		"application/graphql+json",
		"application/yaml",
		"application/x-yaml",
		"application/toml",
		"application/csv",
		"application/sql",
		"application/manifest+json",
		"application/vnd.api+json",
		"application/feed+json":
		return true
	}
	// +json / +xml structured suffixes
	if strings.HasSuffix(mediaType, "+json") || strings.HasSuffix(mediaType, "+xml") || strings.HasSuffix(mediaType, "+yaml") {
		return true
	}
	return false
}

func looksLikeUTF8Text(b []byte) bool {
	if len(b) == 0 {
		return true
	}
	// Sample up to 512 bytes for control-char / invalid UTF-8 ratio.
	sample := b
	if len(sample) > 512 {
		sample = sample[:512]
	}
	if !utf8.Valid(sample) {
		// Allow invalid only if it's truncated mid-rune at the end of full body — here sample only.
		// For detection, require valid UTF-8 on the sample after trimming incomplete trailing rune.
		trim := sample
		for len(trim) > 0 && !utf8.Valid(trim) {
			trim = trim[:len(trim)-1]
		}
		if len(trim) < len(sample)/2 || !utf8.Valid(trim) {
			return false
		}
		sample = trim
	}
	nonPrint := 0
	for i := 0; i < len(sample); {
		r, size := utf8.DecodeRune(sample[i:])
		i += size
		if r == utf8.RuneError && size == 1 {
			return false
		}
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		if r < 0x20 || r == 0x7f {
			nonPrint++
		}
	}
	return nonPrint*20 <= len(sample) // ≤5% C0 controls
}

func applySafeHeaders(req *http.Request, headers map[string]string) error {
	total := 0
	for k, v := range headers {
		if k == "" {
			return errors.New("header name must not be empty")
		}
		if strings.IndexFunc(k, func(r rune) bool {
			return r > unicode.MaxASCII || (!unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_')
		}) >= 0 {
			return fmt.Errorf("invalid header name %q", k)
		}
		if strings.ContainsAny(k, "\r\n") || strings.ContainsAny(v, "\r\n") {
			return errors.New("headers must not contain CR/LF")
		}
		canon := http.CanonicalHeaderKey(k)
		if _, blocked := blockedRequestHeaders[strings.ToLower(canon)]; blocked {
			return fmt.Errorf("header %q is not allowed", canon)
		}
		if strings.HasPrefix(strings.ToLower(canon), "proxy-") {
			return fmt.Errorf("header %q is not allowed", canon)
		}
		total += len(canon) + len(v)
		if total > httpRequestMaxHeaderSz {
			return fmt.Errorf("headers exceed maximum total size of %d bytes", httpRequestMaxHeaderSz)
		}
		req.Header.Set(canon, v)
	}
	return nil
}

func newSafeHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy:                 nil, // never honor HTTP_PROXY from env for this tool
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     true,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			if network != "tcp" && network != "tcp4" && network != "tcp6" {
				return nil, fmt.Errorf("network %q is not allowed", network)
			}
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := resolveDialIPs(ctx, host)
			if err != nil {
				return nil, err
			}
			var lastErr error
			for _, ip := range ips {
				if err := assertPublicIP(ip); err != nil {
					lastErr = err
					continue
				}
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			if lastErr == nil {
				lastErr = errors.New("no public addresses to dial")
			}
			return nil, lastErr
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   httpRequestTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= httpRequestMaxRedirect {
				return fmt.Errorf("stopped after %d redirects", httpRequestMaxRedirect)
			}
			if _, err := validatePublicHTTPSURL(req.URL.String()); err != nil {
				return fmt.Errorf("redirect blocked: %w", err)
			}
			// Drop body-bearing method quirks are handled by net/http; re-apply header policy
			// by stripping blocked headers that servers might have set on the redirect request.
			for h := range blockedRequestHeaders {
				req.Header.Del(h)
			}
			return nil
		},
	}
}

func validatePublicHTTPSURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("url is required")
	}
	if strings.ContainsAny(raw, " \t\r\n") {
		return nil, errors.New("url must not contain whitespace")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "https" {
		return nil, errors.New("only https URLs are allowed")
	}
	if u.Host == "" {
		return nil, errors.New("url host is required")
	}
	if u.User != nil {
		return nil, errors.New("url userinfo is not allowed")
	}
	if u.Opaque != "" {
		return nil, errors.New("opaque urls are not allowed")
	}
	if u.Fragment != "" {
		// Fragments are not sent to servers; drop rather than reject.
		u.Fragment = ""
	}

	host := u.Hostname()
	if host == "" {
		return nil, errors.New("url host is required")
	}
	if err := validateHostname(host); err != nil {
		return nil, err
	}

	port := u.Port()
	if port != "" && port != "443" {
		// Allow non-443 HTTPS only on public hosts (still resolved/checked).
		// Reject clearly local-only ports? No — port alone isn't SSRF if IP is public.
		// But reject port 0 and non-numeric.
		if _, err := net.LookupPort("tcp", port); err != nil {
			return nil, fmt.Errorf("invalid port: %w", err)
		}
	}

	// Resolve and ensure every address is a public IP (blocks DNS→private).
	if err := resolveAndAssertPublic(host); err != nil {
		return nil, err
	}

	return u, nil
}

func validateHostname(host string) error {
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	if h == "" {
		return errors.New("empty hostname")
	}

	// Literal IPs are never allowed — hostname must be a DNS name.
	if ip := net.ParseIP(h); ip != nil {
		return errors.New("IP address hosts are not allowed; use a DNS hostname")
	}
	// Bracketed IPv6 should already be stripped by Hostname(), but be defensive.
	if strings.HasPrefix(h, "[") {
		return errors.New("IP address hosts are not allowed; use a DNS hostname")
	}

	if _, blocked := blockedHostnames[h]; blocked {
		return fmt.Errorf("host %q is not allowed", h)
	}

	// Block obvious local/internal suffixes.
	blockedSuffixes := []string{
		".localhost",
		".local",
		".internal",
		".intranet",
		".corp",
		".home",
		".lan",
		".home.arpa",
		".localdomain",
		".invalid",
		".onion",
	}
	for _, suf := range blockedSuffixes {
		if h == strings.TrimPrefix(suf, ".") || strings.HasSuffix(h, suf) {
			return fmt.Errorf("host suffix %q is not allowed", suf)
		}
	}

	// Basic DNS label sanity (no spaces, no .., length limits).
	if len(h) > 253 {
		return errors.New("hostname too long")
	}
	if strings.Contains(h, "..") || strings.HasPrefix(h, ".") {
		return errors.New("invalid hostname")
	}
	for _, label := range strings.Split(h, ".") {
		if label == "" || len(label) > 63 {
			return errors.New("invalid hostname label")
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return errors.New("invalid hostname label")
		}
		for _, r := range label {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
				continue
			}
			// Allow punycode xn-- already covered by a-z0-9-
			return fmt.Errorf("invalid hostname character in %q", label)
		}
	}

	// Reject decimal/octal/hex IP obfuscation like 2130706433 or 0x7f000001 as host.
	if isObfuscatedIPHost(h) {
		return errors.New("obfuscated IP hosts are not allowed")
	}

	return nil
}

func isObfuscatedIPHost(host string) bool {
	// Single-label all-digit hosts (e.g. 2130706433 → 127.0.0.1)
	if strings.Contains(host, ".") {
		// Dotted forms that aren't valid DNS but parse as IP are already caught by ParseIP.
		// Catch octal-ish 0177.0.0.1 — Go's ParseIP may not parse these; reject labels with leading zeros-only octal patterns for pure-numeric dotted quads.
		parts := strings.Split(host, ".")
		if len(parts) == 4 {
			allNumeric := true
			for _, p := range parts {
				if p == "" {
					allNumeric = false
					break
				}
				for _, c := range p {
					if c < '0' || c > '9' {
						allNumeric = false
						break
					}
				}
				if !allNumeric {
					break
				}
			}
			if allNumeric {
				return true
			}
		}
		return false
	}
	// hex form
	if strings.HasPrefix(host, "0x") || strings.HasPrefix(host, "0X") {
		return true
	}
	// pure integer
	if host[0] >= '0' && host[0] <= '9' {
		allDigit := true
		for _, c := range host {
			if c < '0' || c > '9' {
				allDigit = false
				break
			}
		}
		return allDigit
	}
	return false
}

func resolveAndAssertPublic(host string) error {
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("dns lookup failed for %q: %w", host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("dns lookup returned no addresses for %q", host)
	}
	// Fail closed: every resolved address must be public (blocks dual-stack
	// records that mix public + link-local/private).
	for _, ip := range ips {
		if err := assertPublicIP(ip); err != nil {
			return fmt.Errorf("host %q resolves to blocked address %s: %w", host, ip.String(), err)
		}
	}
	return nil
}

func resolveDialIPs(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		// Dial path may receive an IP from the transport; still enforce policy.
		return []net.IP{ip}, nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("dns lookup failed for %q: %w", host, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("dns lookup returned no addresses for %q", host)
	}
	ips := make([]net.IP, 0, len(addrs))
	for _, a := range addrs {
		ips = append(ips, a.IP)
	}
	return ips, nil
}

func assertPublicIP(ip net.IP) error {
	if ip == nil {
		return errors.New("nil IP")
	}
	if ip.IsUnspecified() {
		return errors.New("unspecified address")
	}
	if ip.IsLoopback() {
		return errors.New("loopback address")
	}
	if ip.IsPrivate() {
		return errors.New("private address")
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return errors.New("link-local address")
	}
	if ip.IsMulticast() {
		return errors.New("multicast address")
	}
	if ip.IsInterfaceLocalMulticast() {
		return errors.New("interface-local multicast address")
	}

	// IPv4-mapped or plain IPv4 extra ranges not fully covered by IsPrivate.
	v4 := ip.To4()
	if v4 != nil {
		// 0.0.0.0/8
		if v4[0] == 0 {
			return errors.New("current network (0.0.0.0/8)")
		}
		// 100.64.0.0/10 CGNAT
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return errors.New("carrier-grade NAT address")
		}
		// 169.254.0.0/16 link-local (also IsLinkLocalUnicast)
		if v4[0] == 169 && v4[1] == 254 {
			return errors.New("link-local address")
		}
		// 192.0.0.0/24 IETF protocol assignments
		if v4[0] == 192 && v4[1] == 0 && v4[2] == 0 {
			return errors.New("IETF protocol assignment address")
		}
		// 192.0.2.0/24 TEST-NET-1, 198.51.100.0/24 TEST-NET-2, 203.0.113.0/24 TEST-NET-3
		if v4[0] == 192 && v4[1] == 0 && v4[2] == 2 {
			return errors.New("documentation address")
		}
		if v4[0] == 198 && v4[1] == 51 && v4[2] == 100 {
			return errors.New("documentation address")
		}
		if v4[0] == 203 && v4[1] == 0 && v4[2] == 113 {
			return errors.New("documentation address")
		}
		// 198.18.0.0/15 benchmarking
		if v4[0] == 198 && (v4[1] == 18 || v4[1] == 19) {
			return errors.New("benchmarking address")
		}
		// 255.255.255.255
		if v4[0] == 255 && v4[1] == 255 && v4[2] == 255 && v4[3] == 255 {
			return errors.New("broadcast address")
		}
	} else {
		// IPv6: unique local fc00::/7
		if len(ip) == net.IPv6len && (ip[0]&0xfe) == 0xfc {
			return errors.New("unique local address")
		}
		// IPv6 documentation 2001:db8::/32
		if len(ip) == net.IPv6len && ip[0] == 0x20 && ip[1] == 0x01 && ip[2] == 0x0d && ip[3] == 0xb8 {
			return errors.New("documentation address")
		}
	}

	return nil
}
