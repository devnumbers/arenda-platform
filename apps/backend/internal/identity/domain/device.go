package domain

// DeviceType classifies the client device a session was opened from, as
// parsed from the User-Agent once at session creation.
type DeviceType string

const (
	DeviceUnknown  DeviceType = "unknown"
	DeviceComputer DeviceType = "computer"
	DevicePhone    DeviceType = "phone"
	DeviceTablet   DeviceType = "tablet"
	DeviceTV       DeviceType = "tv"
)

// DeviceInfo is the parsed client description stored on a session: the
// device class for the icon, the browser with its major version for the
// «Chrome 121» label, and the operating system. The raw User-Agent string is
// kept alongside so the parse can be replayed later without waiting for a
// fresh login.
type DeviceInfo struct {
	DeviceType   DeviceType
	Browser      string
	BrowserMajor int
	OS           string
}
