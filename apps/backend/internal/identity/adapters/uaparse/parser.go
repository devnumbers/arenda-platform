// Package uaparse adapts the uasurfer User-Agent parser to the identity
// DeviceInfoParser port: one parse per session, at creation (issue #728).
package uaparse

import (
	"github.com/LumenResearch/uasurfer"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// Parser parses raw User-Agent strings into domain device descriptions.
type Parser struct{}

// NewParser creates a Parser. The constructor keeps the wiring symmetrical
// with the other identity adapters.
func NewParser() *Parser { return &Parser{} }

// The adapter is the only implementation of the port the identity application
// consumes; the assertion keeps the wiring rename-safe.
var _ identityapp.DeviceInfoParser = (*Parser)(nil)

// Parse maps the parsed User-Agent onto the stored device description. The
// raw User-Agent itself is persisted by the application alongside this
// result, so a future re-parse never has to wait for fresh logins.
func (p *Parser) Parse(userAgent string) domain.DeviceInfo {
	ua := uasurfer.Parse(userAgent)

	info := domain.DeviceInfo{
		DeviceType:   deviceType(ua.DeviceType),
		Browser:      trimUnknown(ua.Browser.Name.StringTrimPrefix()),
		BrowserMajor: ua.Browser.Version.Major,
		OS:           trimUnknown(ua.OS.Name.StringTrimPrefix()),
	}
	return info
}

// deviceType folds the uasurfer device classes onto the domain vocabulary:
// wearables present as phones, consoles as TVs, everything exotic stays
// unknown.
func deviceType(t uasurfer.DeviceType) domain.DeviceType {
	switch t {
	case uasurfer.DeviceComputer:
		return domain.DeviceComputer
	case uasurfer.DevicePhone, uasurfer.DeviceWearable:
		return domain.DevicePhone
	case uasurfer.DeviceTablet:
		return domain.DeviceTablet
	case uasurfer.DeviceTV, uasurfer.DeviceConsole:
		return domain.DeviceTV
	default:
		return domain.DeviceUnknown
	}
}

// trimUnknown replaces the library's "Unknown" label with an empty string —
// the column default — so unparsed fields stay empty in the API response.
func trimUnknown(s string) string {
	if s == "Unknown" {
		return ""
	}
	return s
}
