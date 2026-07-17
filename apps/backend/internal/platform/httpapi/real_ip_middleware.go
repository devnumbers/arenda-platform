package httpapi

import (
	"net"
	"net/http"
	"slices"
	"strings"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/requestctx"
)

// realIPMiddleware returns a middleware that extracts the client IP from
// X-Forwarded-For or X-Real-IP headers when the immediate remote address is a
// trusted proxy. It replaces chi's deprecated middleware.RealIP with an
// explicit, proxy-aware strategy. The plain host form of the extracted IP is
// also stored in the request context for audit logging.
func realIPMiddleware(trusted []string) func(http.Handler) http.Handler {
	trustedNets := make([]*net.IPNet, 0, len(trusted))
	for _, cidr := range trusted {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			// Configuration is validated at startup; ignore invalid entries here.
			continue
		}
		trustedNets = append(trustedNets, n)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractClientIP(r, trustedNets)
			r.RemoteAddr = ip
			// extractClientIP may keep the original host:port for direct
			// connections; the context carries the plain host form.
			ctxIP := ip
			if host, _, err := net.SplitHostPort(ip); err == nil {
				ctxIP = host
			}
			next.ServeHTTP(w, r.WithContext(requestctx.WithClientIP(r.Context(), ctxIP)))
		})
	}
}

func extractClientIP(r *http.Request, trustedNets []*net.IPNet) string {
	remoteIP, remotePort := parseRemoteAddr(r.RemoteAddr)
	if remoteIP == nil {
		// Cannot parse the remote address; leave it unchanged.
		return r.RemoteAddr
	}

	if !ipInNets(remoteIP, trustedNets) {
		return formatAddr(remoteIP, remotePort)
	}

	chosenIP := pickUntrustedIP(r.Header.Get("X-Forwarded-For"), trustedNets)
	if chosenIP == nil {
		chosenIP = parseSingleIP(r.Header.Get("X-Real-IP"))
	}
	if chosenIP == nil {
		// Every X-Forwarded-For IP is trusted and X-Real-IP is absent;
		// fall back to the rightmost X-Forwarded-For IP.
		chosenIP = rightmostParseableIP(r.Header.Get("X-Forwarded-For"))
	}
	if chosenIP == nil {
		// Trusted proxy but no usable forwarded header; fall back to the proxy's IP.
		return formatAddr(remoteIP, remotePort)
	}
	// A forwarded client IP has no meaningful TCP port; return the bare IP.
	return chosenIP.String()
}

func parseRemoteAddr(addr string) (net.IP, string) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		// Try parsing as a bare IP.
		ip := net.ParseIP(strings.TrimSpace(addr))
		return ip, ""
	}
	return net.ParseIP(host), port
}

func parseSingleIP(s string) net.IP {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return net.ParseIP(s)
}

func pickUntrustedIP(xff string, trustedNets []*net.IPNet) net.IP {
	if xff == "" {
		return nil
	}
	parts := strings.Split(xff, ",")
	// Search from right to left for the first untrusted IP.
	for _, part := range slices.Backward(parts) {
		ip := parseSingleIP(part)
		if ip == nil {
			continue
		}
		if !ipInNets(ip, trustedNets) {
			return ip
		}
	}
	return nil
}

func rightmostParseableIP(xff string) net.IP {
	if xff == "" {
		return nil
	}
	parts := strings.Split(xff, ",")
	for _, part := range slices.Backward(parts) {
		if ip := parseSingleIP(part); ip != nil {
			return ip
		}
	}
	return nil
}

func ipInNets(ip net.IP, nets []*net.IPNet) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func formatAddr(ip net.IP, port string) string {
	if port == "" {
		return ip.String()
	}
	return net.JoinHostPort(ip.String(), port)
}
