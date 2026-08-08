package middleware

import (
	"net"
	"net/http"
	"strings"
)

var trustedNets []*net.IPNet

func init() {
	cidrs := []string{
		"127.0.0.0/8",    // Localhost IPv4
		"::1/128",        // Localhost IPv6
		"10.0.0.0/8",     // Private class A (Docker/Internal)
		"172.16.0.0/12",  // Private class B (Docker networks)
		"192.168.0.0/16", // Private class C
		"fc00::/7",       // IPv6 Unique Local
	}

	for _, cidr := range cidrs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			trustedNets = append(trustedNets, ipNet)
		}
	}
}

// isTrustedIP checks if the given IP address string belongs to a trusted network/proxy.
func isTrustedIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, network := range trustedNets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// TrustedRealIP returns an HTTP middleware that extracts client IP safely.
// Header X-Forwarded-For dan X-Real-IP hanya dipercayai jika koneksi socket (r.RemoteAddr)
// berasal dari Trusted Proxy (misal: Nginx dalam jaringan Docker/Localhost).
func TrustedRealIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peerHost, port, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			peerHost = r.RemoteAddr
			port = ""
		}

		// Jika r.RemoteAddr berasal dari trusted proxy (misal Nginx container),
		// ambil IP sebenarnya dari X-Real-IP atau X-Forwarded-For.
		if isTrustedIP(peerHost) {
			var realIP string

			if xri := r.Header.Get("X-Real-IP"); xri != "" {
				realIP = strings.TrimSpace(xri)
			} else if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				// Ambil IP terluar (paling kiri) dari daftar XFF jika dipercayai
				parts := strings.Split(xff, ",")
				for i := len(parts) - 1; i >= 0; i-- {
					p := strings.TrimSpace(parts[i])
					if p != "" && !isTrustedIP(p) {
						realIP = p
						break
					}
				}
				if realIP == "" && len(parts) > 0 {
					realIP = strings.TrimSpace(parts[0])
				}
			}

			if realIP != "" && net.ParseIP(realIP) != nil {
				if port != "" {
					r.RemoteAddr = net.JoinHostPort(realIP, port)
				} else {
					r.RemoteAddr = realIP
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}
