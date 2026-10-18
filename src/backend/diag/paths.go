package diag

// Paths are the settings files of the router the checks read; a test has its own.
type Paths struct {
	Settings string
	DHCP     string
	Network  string
}

// RouterPaths are the files of an OpenWrt router.
var RouterPaths = Paths{Settings: "/etc/config/podkop", DHCP: "/etc/config/dhcp", Network: "/etc/config/network"}
