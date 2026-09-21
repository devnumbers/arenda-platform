package geoip

import (
	"context"
	"log/slog"
	"net"
	"os"
	"testing"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
)

// testDatabase writes a tiny in-memory city database (kilobytes, no real data)
// covering a few networks, and returns the file path. It emulates the DB-IP
// City Lite record shape the resolver reads (city.names with ru/en).
func testDatabase(t *testing.T) string {
	t.Helper()
	tree, err := mmdbwriter.New(mmdbwriter.Options{
		DatabaseType: "DBIP-City-Lite",
		Languages:    []string{"en", "ru"},
		// TEST-NET ranges (81.2.69.0/24 is public, 203.0.113.0/24 and
		// 198.51.100.0/24 are documentation ranges) need this flag.
		IncludeReservedNetworks: true,
	})
	if err != nil {
		t.Fatalf("build mmdb tree: %v", err)
	}
	cityRecord := func(en, ru, iso string) mmdbtype.DataType {
		return mmdbtype.Map{
			"city": mmdbtype.Map{
				"names": mmdbtype.Map{
					"en": mmdbtype.String(en),
					"ru": mmdbtype.String(ru),
				},
			},
			"country": mmdbtype.Map{
				"iso_code": mmdbtype.String(iso),
				"names": mmdbtype.Map{
					"en": mmdbtype.String("Testland"),
					"ru": mmdbtype.String("Тестландия"),
				},
			},
		}
	}
	// Network parses a CIDR into a *net.IPNet for the tree insert.
	network := func(cidr string) *net.IPNet {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			t.Fatalf("parse %s: %v", cidr, err)
		}
		return ipnet
	}
	// Rows seed the fixture networks. Enonly's city has no Russian name, so
	// its lookup answers with the English fallback.
	rows := []struct {
		CIDR string
		Rec  mmdbtype.DataType
	}{
		{CIDR: "81.2.69.0/24", Rec: cityRecord("London", "Лондон", "GB")},
		{CIDR: "203.0.113.0/24", Rec: cityRecord("Testville", "Тестоград", "XX")},
		{CIDR: "198.51.100.0/24", Rec: cityRecord("Enonly", "Enonly", "YY")},
	}
	for _, tc := range rows {
		if err := tree.Insert(network(tc.CIDR), tc.Rec); err != nil {
			t.Fatalf("insert %s: %v", tc.CIDR, err)
		}
	}
	f, err := os.CreateTemp(t.TempDir(), "city-lite.mmdb")
	if err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	path := f.Name()
	defer func() {
		if cerr := f.Close(); cerr != nil {
			t.Logf("close fixture database: %v", cerr)
		}
	}()
	if _, err := tree.WriteTo(f); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestResolver_ResolvesCityRuWithEnFallback(t *testing.T) {
	t.Parallel()
	r := NewResolver(t.Context(), testDatabase(t), slog.New(slog.DiscardHandler))
	ctx := context.Background()

	if city, ok := r.ResolveCity(ctx, "81.2.69.142"); !ok || city != "Лондон" {
		t.Fatalf("ResolveCity(81.2.69.142) = %q,%v; want Лондон,true", city, ok)
	}
	// No Russian name in the record — the English one answers.
	if city, ok := r.ResolveCity(ctx, "198.51.100.5"); !ok || city != "Enonly" {
		t.Fatalf("ResolveCity(198.51.100.5) = %q,%v; want Enonly,true (en fallback)", city, ok)
	}
	// Unknown network inside the public space.
	if city, ok := r.ResolveCity(ctx, "192.0.2.1"); ok {
		t.Fatalf("ResolveCity(192.0.2.1) = %q,true; want not found", city)
	}
}

func TestResolver_SkipsPrivateAndSpecialIPs(t *testing.T) {
	t.Parallel()
	r := NewResolver(t.Context(), testDatabase(t), slog.New(slog.DiscardHandler))
	ctx := context.Background()

	// Addresses geolocation must not answer for: loopback, RFC 1918 private
	// space, CGNAT, link-local, IPv6 loopback and ULA, the absent value, a
	// garbage string, and the unspecified address.
	unresolvable := []string{
		"127.0.0.1",
		"10.20.30.40",
		"192.168.1.1",
		"100.64.0.9",
		"169.254.1.1",
		"::1",
		"fd12:3456::1",
		"",
		"not-an-ip",
		"0.0.0.0",
	}
	for _, ip := range unresolvable {
		if city, ok := r.ResolveCity(ctx, ip); ok {
			t.Errorf("ResolveCity(%q) = %q,true; want not resolved", ip, city)
		}
	}
}

func TestResolver_MissingDatabaseDegrades(t *testing.T) {
	t.Parallel()
	// A resolver whose database failed to open stays fully functional: every
	// lookup reports "not found" and the application stores no city.
	r := NewResolver(t.Context(), "/nonexistent/city.mmdb", slog.New(slog.DiscardHandler))
	if city, ok := r.ResolveCity(context.Background(), "81.2.69.142"); ok {
		t.Fatalf("ResolveCity with a broken base = %q,true; want not resolved", city)
	}
}
