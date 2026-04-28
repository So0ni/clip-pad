package middleware

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type realIPContextKey struct{}

type RealIPResolver struct {
	trustCloudflare   bool
	trustProxyHeaders bool
	trustedProxies    []netip.Prefix
}

func NewRealIPResolver(trustCloudflare, trustProxyHeaders bool, trusted []netip.Prefix) *RealIPResolver {
	if len(trusted) == 0 {
		trusted = defaultCloudflareCIDRs()
	}
	return &RealIPResolver{
		trustCloudflare:   trustCloudflare,
		trustProxyHeaders: trustProxyHeaders,
		trustedProxies:    trusted,
	}
}

func (r *RealIPResolver) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		realIP := r.Resolve(req)
		ctx := context.WithValue(req.Context(), realIPContextKey{}, realIP)
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

func GetRealIP(req *http.Request) string {
	if value, ok := req.Context().Value(realIPContextKey{}).(string); ok && value != "" {
		return value
	}
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err == nil {
		return host
	}
	return req.RemoteAddr
}

func (r *RealIPResolver) Resolve(req *http.Request) string {
	sourceIP, ok := remoteAddrIP(req.RemoteAddr)
	if !ok {
		return req.RemoteAddr
	}

	if r.trustCloudflare && r.isTrustedSource(sourceIP) {
		if cfIP, ok := parseSingleIP(req.Header.Get("CF-Connecting-IP")); ok {
			return cfIP.String()
		}
	}

	if r.trustProxyHeaders && r.isTrustedSource(sourceIP) {
		if forwardedIP, ok := firstForwardedFor(req.Header.Get("X-Forwarded-For")); ok {
			return forwardedIP.String()
		}
	}

	return sourceIP.String()
}

func (r *RealIPResolver) isTrustedSource(ip netip.Addr) bool {
	for _, prefix := range r.trustedProxies {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func remoteAddrIP(remoteAddr string) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return parseSingleIP(host)
}

func parseSingleIP(value string) (netip.Addr, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return netip.Addr{}, false
	}
	ip, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}, false
	}
	return ip.Unmap(), true
}

func firstForwardedFor(value string) (netip.Addr, bool) {
	parts := strings.Split(value, ",")
	if len(parts) == 0 {
		return netip.Addr{}, false
	}
	return parseSingleIP(parts[0])
}

func defaultCloudflareCIDRs() []netip.Prefix {
	cidrs := []string{
		"173.245.48.0/20",
		"103.21.244.0/22",
		"103.22.200.0/22",
		"103.31.4.0/22",
		"141.101.64.0/18",
		"108.162.192.0/18",
		"190.93.240.0/20",
		"188.114.96.0/20",
		"197.234.240.0/22",
		"198.41.128.0/17",
		"162.158.0.0/15",
		"104.16.0.0/13",
		"104.24.0.0/14",
		"172.64.0.0/13",
		"131.0.72.0/22",
		"2400:cb00::/32",
		"2606:4700::/32",
		"2803:f800::/32",
		"2405:b500::/32",
		"2405:8100::/32",
		"2a06:98c0::/29",
		"2c0f:f248::/32",
	}
	prefixes := make([]netip.Prefix, 0, len(cidrs))
	for _, cidr := range cidrs {
		if prefix, err := netip.ParsePrefix(cidr); err == nil {
			prefixes = append(prefixes, prefix)
		}
	}
	return prefixes
}
