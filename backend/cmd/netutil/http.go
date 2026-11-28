// Package netutil provides outbound HTTP clients with SSRF protections.
package netutil

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const maxRedirects = 3

var blockedHostnames = map[string]struct{}{
	"localhost":                            {},
	"localhost.localdomain":                {},
	"metadata":                             {},
	"metadata.google.internal":             {},
	"metadata.goog":                        {},
	"kubernetes":                           {},
	"kubernetes.default":                   {},
	"kubernetes.default.svc":               {},
	"kubernetes.default.svc.cluster.local": {},
}

// LocalNetworkAllowed reports whether the operator explicitly allowed HTTP and
// private-network provider/MCP endpoints. It is checked for every request so
// tests and long-running processes observe the current environment.
func LocalNetworkAllowed() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ALLOW_LOCAL_NETWORK"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// NewPublicHTTPSClient returns a client that only connects to public DNS hosts
// over HTTPS. ALLOW_LOCAL_NETWORK deliberately does not relax this policy.
func NewPublicHTTPSClient(timeout time.Duration) *http.Client {
	return newClient(timeout, false)
}

// NewOutboundHTTPClient returns a client for user-configured provider and MCP
// endpoints. It is public-HTTPS-only unless ALLOW_LOCAL_NETWORK=true.
func NewOutboundHTTPClient(timeout time.Duration) *http.Client {
	return newClient(timeout, true)
}

type policyTransport struct {
	base          http.RoundTripper
	allowLocalOpt bool
}

func (t *policyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if _, err := validateForPolicy(req.URL.String(), t.allowLocalOpt && LocalNetworkAllowed()); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(req)
}

func newClient(timeout time.Duration, allowLocalOpt bool) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			if network != "tcp" && network != "tcp4" && network != "tcp6" {
				return nil, fmt.Errorf("network %q is not allowed", network)
			}
			allowLocal := allowLocalOpt && LocalNetworkAllowed()
			if allowLocal {
				return dialer.DialContext(ctx, network, address)
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
				if err := AssertPublicIP(ip); err != nil {
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
		Transport: &policyTransport{base: transport, allowLocalOpt: allowLocalOpt},
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			if _, err := validateForPolicy(req.URL.String(), allowLocalOpt && LocalNetworkAllowed()); err != nil {
				return fmt.Errorf("redirect blocked: %w", err)
			}
			return nil
		},
	}
}

// ValidatePublicHTTPSURL validates and canonicalizes a public HTTPS URL.
func ValidatePublicHTTPSURL(raw string) (*url.URL, error) {
	return validateForPolicy(raw, false)
}

// ValidateOutboundURL applies the provider/MCP URL policy.
func ValidateOutboundURL(raw string) (*url.URL, error) {
	return validateForPolicy(raw, LocalNetworkAllowed())
}

func validateForPolicy(raw string, allowLocal bool) (*url.URL, error) {
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
	if allowLocal {
		if u.Scheme != "https" && u.Scheme != "http" {
			return nil, errors.New("only http and https URLs are allowed")
		}
	} else if u.Scheme != "https" {
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
	u.Fragment = ""

	host := u.Hostname()
	if host == "" {
		return nil, errors.New("url host is required")
	}
	if !allowLocal {
		if err := ValidateHostname(host); err != nil {
			return nil, err
		}
	}

	if port := u.Port(); port != "" && port != "443" {
		if _, err := net.LookupPort("tcp", port); err != nil {
			return nil, fmt.Errorf("invalid port: %w", err)
		}
	}
	if !allowLocal {
		if err := resolveAndAssertPublic(host); err != nil {
			return nil, err
		}
	}
	return u, nil
}

// ValidateHostname rejects IP literals, local names, and malformed DNS names.
func ValidateHostname(host string) error {
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	if h == "" {
		return errors.New("empty hostname")
	}
	if ip := net.ParseIP(h); ip != nil {
		return errors.New("IP address hosts are not allowed; use a DNS hostname")
	}
	if strings.HasPrefix(h, "[") {
		return errors.New("IP address hosts are not allowed; use a DNS hostname")
	}
	if _, blocked := blockedHostnames[h]; blocked {
		return fmt.Errorf("host %q is not allowed", h)
	}

	blockedSuffixes := []string{
		".localhost", ".local", ".internal", ".intranet", ".corp", ".home",
		".lan", ".home.arpa", ".localdomain", ".invalid", ".onion",
	}
	for _, suf := range blockedSuffixes {
		if h == strings.TrimPrefix(suf, ".") || strings.HasSuffix(h, suf) {
			return fmt.Errorf("host suffix %q is not allowed", suf)
		}
	}
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
			return fmt.Errorf("invalid hostname character in %q", label)
		}
	}
	if IsObfuscatedIPHost(h) {
		return errors.New("obfuscated IP hosts are not allowed")
	}
	return nil
}

// IsObfuscatedIPHost detects numeric and hexadecimal IP obfuscation.
func IsObfuscatedIPHost(host string) bool {
	if strings.Contains(host, ".") {
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
	if strings.HasPrefix(host, "0x") || strings.HasPrefix(host, "0X") {
		return true
	}
	if host != "" && host[0] >= '0' && host[0] <= '9' {
		for _, c := range host {
			if c < '0' || c > '9' {
				return false
			}
		}
		return true
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
	for _, ip := range ips {
		if err := AssertPublicIP(ip); err != nil {
			return fmt.Errorf("host %q resolves to blocked address %s: %w", host, ip.String(), err)
		}
	}
	return nil
}

func resolveDialIPs(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
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

// AssertPublicIP rejects non-public and special-purpose IP addresses.
func AssertPublicIP(ip net.IP) error {
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
	if ip.IsMulticast() || ip.IsInterfaceLocalMulticast() {
		return errors.New("multicast address")
	}

	v4 := ip.To4()
	if v4 != nil {
		if v4[0] == 0 {
			return errors.New("current network (0.0.0.0/8)")
		}
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return errors.New("carrier-grade NAT address")
		}
		if v4[0] == 169 && v4[1] == 254 {
			return errors.New("link-local address")
		}
		if v4[0] == 192 && v4[1] == 0 && v4[2] == 0 {
			return errors.New("IETF protocol assignment address")
		}
		if v4[0] == 192 && v4[1] == 0 && v4[2] == 2 {
			return errors.New("documentation address")
		}
		if v4[0] == 198 && v4[1] == 51 && v4[2] == 100 {
			return errors.New("documentation address")
		}
		if v4[0] == 203 && v4[1] == 0 && v4[2] == 113 {
			return errors.New("documentation address")
		}
		if v4[0] == 198 && (v4[1] == 18 || v4[1] == 19) {
			return errors.New("benchmarking address")
		}
		if v4[0] == 255 && v4[1] == 255 && v4[2] == 255 && v4[3] == 255 {
			return errors.New("broadcast address")
		}
	} else {
		if len(ip) == net.IPv6len && (ip[0]&0xfe) == 0xfc {
			return errors.New("unique local address")
		}
		if len(ip) == net.IPv6len && ip[0] == 0x20 && ip[1] == 0x01 && ip[2] == 0x0d && ip[3] == 0xb8 {
			return errors.New("documentation address")
		}
	}
	return nil
}
