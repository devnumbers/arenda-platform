// Package geoip adapts the offline DB-IP City Lite database (read through
// oschwald/geoip2-golang) to the identity GeoResolver port: the city for a
// client IP, resolved in the ru locale with en as the fallback (issue #728).
//
// The database file is baked into the Docker image and refreshed by the
// monthly image rebuild; a missing or broken database degrades the resolver
// to "never resolves" — the application stores no city and the product keeps
// working (mini-ADR «Геолокация города — в приложении, не на грани»).
package geoip

import (
	"context"
	"log/slog"
	"net/netip"

	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/oschwald/geoip2-golang/v2"
)

// Resolver answers city lookups from the local mmdb file.
type Resolver struct {
	reader *geoip2.Reader
	logger *slog.Logger
}

// NewResolver opens the city database at path. An open failure is not fatal:
// it is logged once and the resolver answers "not found" for every lookup,
// keeping the application fully functional without geolocation.
func NewResolver(ctx context.Context, path string, logger *slog.Logger) *Resolver {
	if logger == nil {
		logger = slog.Default()
	}
	reader, err := geoip2.Open(path)
	if err != nil {
		logger.WarnContext(ctx, "geoip database unavailable, cities will not be resolved",
			slog.String("path", path), slog.String("error", err.Error()))
		return &Resolver{logger: logger}
	}
	return &Resolver{reader: reader, logger: logger}
}

// The adapter is the only implementation of the port the identity application
// consumes; the assertion keeps the wiring rename-safe.
var _ identityapp.GeoResolver = (*Resolver)(nil)

// ResolveCity returns the city name for a public client IP. Private, reserved,
// and unknown IPs report found=false; an existing stored city is never erased
// by such an answer (the application keeps the previous value).
func (r *Resolver) ResolveCity(ctx context.Context, ip string) (string, bool) {
	if r.reader == nil || ip == "" {
		return "", false
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil || !isPublic(addr) {
		return "", false
	}

	record, err := r.reader.City(addr)
	if err != nil {
		r.logger.DebugContext(ctx, "geoip lookup failed", slog.String("error", err.Error()))
		return "", false
	}
	if record.City.Names.Russian != "" {
		return record.City.Names.Russian, true
	}
	if record.City.Names.English != "" {
		return record.City.Names.English, true
	}
	return "", false
}

// isPublic filters the addresses geolocation cannot meaningfully answer for:
// loopback, RFC1918/ULA private space, CGNAT, link-local, multicast, and the
// unspecified address.
func isPublic(addr netip.Addr) bool {
	if !addr.IsValid() || addr.IsUnspecified() || addr.IsLoopback() ||
		addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() ||
		addr.IsMulticast() {
		return false
	}
	// CGNAT 100.64.0.0/10 has no netip predicate.
	if addr.Is4() {
		octets := addr.As4()
		return octets[0] != 100 || octets[1] < 64 || octets[1] > 127
	}
	return true
}
