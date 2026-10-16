package uaparse

import (
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// Canonical User-Agent strings with their expected parse — the expectations
// pin the domain mapping (device class, browser + major, OS), not the
// library internals.
func TestParser_Parse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		userAgent      string
		wantType       domain.DeviceType
		wantBrowser    string
		wantBrowserMid int
		wantOS         string
	}{
		{
			name: "Chrome on Windows",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
				"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
			wantType:       domain.DeviceComputer,
			wantBrowser:    "Chrome",
			wantBrowserMid: 121,
			wantOS:         "Windows",
		},
		{
			name: "Yandex Browser on Windows",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
				"(KHTML, like Gecko) Chrome/120.0.0.0 YaBrowser/24.1.0.0 Safari/537.36",
			wantType:       domain.DeviceComputer,
			wantBrowser:    "Yandex",
			wantBrowserMid: 24,
			wantOS:         "Windows",
		},
		{
			name: "Safari on iPhone",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_3 like Mac OS X) AppleWebKit/605.1.15 " +
				"(KHTML, like Gecko) Version/17.3 Mobile/15E148 Safari/604.1",
			wantType:       domain.DevicePhone,
			wantBrowser:    "Safari",
			wantBrowserMid: 17,
			wantOS:         "iOS",
		},
		{
			name: "Chrome on Android phone",
			userAgent: "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 " +
				"(KHTML, like Gecko) Chrome/121.0.6167.101 Mobile Safari/537.36",
			wantType:       domain.DevicePhone,
			wantBrowser:    "Chrome",
			wantBrowserMid: 121,
			wantOS:         "Android",
		},
		{
			name: "Safari on iPad is a tablet",
			userAgent: "Mozilla/5.0 (iPad; CPU OS 17_3 like Mac OS X) AppleWebKit/605.1.15 " +
				"(KHTML, like Gecko) Version/17.3 Mobile/15E148 Safari/604.1",
			wantType:       domain.DeviceTablet,
			wantBrowser:    "Safari",
			wantBrowserMid: 17,
			wantOS:         "iPadOS",
		},
		{
			name:           "empty User-Agent degrades to unknown",
			userAgent:      "",
			wantType:       domain.DeviceUnknown,
			wantBrowser:    "",
			wantBrowserMid: 0,
			wantOS:         "",
		},
	}

	parser := NewParser()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parser.Parse(tc.userAgent)
			if got.DeviceType != tc.wantType {
				t.Errorf("DeviceType = %q, want %q", got.DeviceType, tc.wantType)
			}
			if got.Browser != tc.wantBrowser {
				t.Errorf("Browser = %q, want %q", got.Browser, tc.wantBrowser)
			}
			if got.BrowserMajor != tc.wantBrowserMid {
				t.Errorf("BrowserMajor = %d, want %d", got.BrowserMajor, tc.wantBrowserMid)
			}
			if got.OS != tc.wantOS {
				t.Errorf("OS = %q, want %q", got.OS, tc.wantOS)
			}
		})
	}
}
