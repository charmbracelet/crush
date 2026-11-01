package tools

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

var cgnatNet = func() *net.IPNet {
	_, network, _ := net.ParseCIDR("100.64.0.0/10")
	return network
}()

// isPublicIP reports whether ip is routable on the public internet.
func isPublicIP(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	// Shared address space is not public.
	if cgnatNet != nil && cgnatNet.Contains(ip) {
		return false
	}
	return true
}

// validateFetchURL rejects URLs whose host resolves to a non-public address.
func validateFetchURL(ctx context.Context, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL has no host")
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return fmt.Errorf("URL host %q resolves to a non-public address", host)
		}
		return nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("cannot resolve URL host %q: %w", host, err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("URL host %q has no addresses", host)
	}
	for _, addr := range addrs {
		if !isPublicIP(addr.IP) {
			return fmt.Errorf("URL host %q resolves to a non-public address", host)
		}
	}
	return nil
}

// checkFetchRedirect re-validates every redirect target before following it.
func checkFetchRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	return validateFetchURL(req.Context(), req.URL.String())
}

// withSafeFetchRedirects clones client with redirect targets validated.
func withSafeFetchRedirects(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	clone := *client
	clone.CheckRedirect = checkFetchRedirect
	return &clone
}
