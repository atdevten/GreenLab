package webhook

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"time"
)

// Client implements application.WebhookClient using net/http.
// All outbound requests are validated:
//   - HTTPS scheme only
//   - Destinations must not resolve to RFC 1918, link-local, or loopback addresses
type Client struct {
	httpClient *http.Client
}

// privateRanges lists IP ranges that must not be targeted by webhooks.
var privateRanges = []netip.Prefix{
	netip.MustParsePrefix("127.0.0.0/8"),    // IPv4 loopback
	netip.MustParsePrefix("10.0.0.0/8"),     // RFC 1918
	netip.MustParsePrefix("172.16.0.0/12"),  // RFC 1918
	netip.MustParsePrefix("192.168.0.0/16"), // RFC 1918
	netip.MustParsePrefix("169.254.0.0/16"), // link-local (AWS IMDS, Azure IMDS, etc.)
	netip.MustParsePrefix("100.64.0.0/10"),  // shared address space (RFC 6598)
	netip.MustParsePrefix("::1/128"),        // IPv6 loopback
	netip.MustParsePrefix("fc00::/7"),       // IPv6 unique local
	netip.MustParsePrefix("fe80::/10"),      // IPv6 link-local
}

// isPrivateIP reports whether ip is in any blocked range.
func isPrivateIP(ip netip.Addr) bool {
	ip = ip.Unmap() // normalise IPv4-mapped IPv6 addresses (::ffff:x.x.x.x → x.x.x.x)
	for _, prefix := range privateRanges {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

// NewClient creates a new webhook Client with SSRF protection.
// The custom DialContext resolves each hostname at connection time and rejects
// any address in a private/internal range. This prevents TOCTOU DNS-rebinding
// attacks that would bypass a pre-flight URL check.
func NewClient() *Client {
	baseDialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("invalid address %q: %w", addr, err)
			}
			ips, err := net.DefaultResolver.LookupHost(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("DNS lookup for %q failed: %w", host, err)
			}
			for _, raw := range ips {
				ip, parseErr := netip.ParseAddr(raw)
				if parseErr != nil {
					continue
				}
				if isPrivateIP(ip) {
					return nil, fmt.Errorf("webhook destination %q resolves to blocked address %s", host, raw)
				}
			}
			return baseDialer.DialContext(ctx, network, net.JoinHostPort(ips[0], port))
		},
	}
	return &Client{
		httpClient: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
		},
	}
}

// Post sends a JSON payload to the given URL via HTTP POST.
// Returns an error if the URL is not HTTPS, resolves to a private address,
// the request fails, or the server returns a non-2xx status.
func (c *Client) Post(ctx context.Context, rawURL, payload string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid webhook URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("webhook URL must use https, got %q", u.Scheme)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL,
		bytes.NewBufferString(payload))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GreenLab-Webhook/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()
	// Drain so the underlying TCP connection can be reused.
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned non-2xx status: %d", resp.StatusCode)
	}
	return nil
}
