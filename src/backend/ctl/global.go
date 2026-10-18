package ctl

import (
	"context"
	"net/netip"
	"os"
	"strings"
	"sync"

	"github.com/itdoginfo/podkop/src/backend/dataplane"
	"github.com/itdoginfo/podkop/src/backend/diag"
	"github.com/itdoginfo/podkop/src/backend/model"
	"github.com/itdoginfo/podkop/src/backend/uci"
)

// compat: page-answers
// The report of global_check is the shell backend's, line for line: people paste it into
// bug reports, and those who read them know it. The checks themselves are in diag; this is
// their text.

const rule = "━━━━━━━━━━━━━━━━━━━━━━━━━━━"

// the first two octets of the addresses WARP endpoints have
var warpPrefixes = []string{"8.47", "162.159", "188.114"}

func mark(ok bool, good string, bad string) string {
	if ok {
		return good
	}
	return bad
}

func flag(value bool, text string) string {
	return mark(value, "✅ ", "❌ ") + text
}

func counted(value bool, text string) string {
	return mark(value, "✅ ", "⚠️  ") + text
}

// globalCheck runs every check and writes the report the user interface shows and people
// paste into bug reports: the text of the shell backend's report, line for line, with the
// settings masked. The settings are those the backend reads (model.Load), with its
// defaults; the file of paths.Settings is shown as it is. Running tells whether the
// backend runs. routerFakeIP asks the backend to check the router itself with a check
// name: whether that worked, and whether the backend answered at all.
func globalCheck(ctx context.Context, paths diag.Paths, settings *model.Settings, version string, singBoxVersion string, running bool, routerFakeIP func() (works bool, answered bool)) string {
	var lines []string
	add := func(text ...string) { lines = append(lines, text...) }

	// What asks the network is asked at once, each part waiting for its own answer: one
	// after another they add up to several seconds on a slow link.
	settingsContent, _ := os.ReadFile(paths.Settings)
	dhcpContent, _ := os.ReadFile(paths.DHCP)
	dhcp, _ := uci.Parse(dhcpContent)
	if settings == nil {
		settings = new(model.Settings)
	}
	var (
		system          *diag.System
		dns             *diag.DNS
		tables          *diag.Firewall
		works, answered bool
		address         string
		asked           sync.WaitGroup
	)
	for _, ask := range []func(){
		func() { system = diag.SystemInfo(ctx, version, singBoxVersion) },
		func() {
			dns = diag.CheckDNS(ctx, diag.DNSSettings{Type: settings.DNSType, Server: settings.DNSServer, Bootstrap: settings.BootstrapDNSServer}, dhcp)
		},
		func() { tables = diag.CheckFirewall(ctx, nil) },
		func() { works, answered = routerFakeIP() },
		func() { address = diag.FakeIPAddress(ctx) },
	} {
		asked.Add(1)
		go func() {
			defer asked.Done()
			ask()
		}()
	}
	asked.Wait()

	add("📡 Global check run!", rule, "🛠️ System info")
	add("🕳️ Podkop:        "+system.PodkopVersion+" (latest: "+system.PodkopLatestVersion+")",
		"🕳️ LuCI App:      "+system.LuCIAppVersion,
		"📦 Sing-box:      "+system.SingBoxVersion,
		"🛜 OpenWrt:       "+system.OpenWrtVersion,
		"🛜 Device:        "+system.DeviceModel)

	add(rule, "➡️ DNS status")
	if dns.Bootstrap != "" {
		add(flag(dns.BootstrapAnswers, "Bootstrap DNS: "+dns.Bootstrap))
	}
	add(flag(dns.ServerAnswers, "Main DNS: "+dns.Server+" ["+dns.Type+"]"), flag(dns.RouterAnswers, "DNS on router"))
	switch {
	case settings.DontTouchDHCP:
		add("⚠️ dont_touch_dhcp is enabled. 📄 DHCP config:")
		add(blocks(string(dhcpContent), func(fields []string) bool { return fields[1] == "dnsmasq" })...)
	case !dns.DHCPAsksBackend:
		add("❌ DHCP configuration differs from template. 📄 DHCP config:")
		add(blocks(string(dhcpContent), func(fields []string) bool { return fields[1] == "dnsmasq" })...)
	default:
		add("✅ /etc/config/dhcp")
	}

	add(rule, "📦 Sing-box status")
	// sing-box is a part of the backend: installed, of a version that fits, without a
	// service of its own by construction (singBoxAnswer)
	singBox := diag.CheckSingBox(running)
	add(flag(true, "Sing-box installed"),
		"✅ Sing-box version is compatible (newer than 1.12.4)",
		flag(true, "Sing-box service exist"),
		flag(true, "Sing-box autostart disabled"),
		flag(singBox.Running, "Sing-box process running"),
		flag(singBox.Listening, "Sing-box listening ports"))

	add(rule, "🧱 NFT rules status")
	add(flag(tables.Table, "Table exist"),
		flag(tables.Mangle, "Rules mangle exist"), counted(tables.MangleCounted, "Rules mangle counters"),
		flag(tables.MangleOutput, "Rules mangle output exist"), counted(tables.MangleOutputCounted, "Rules mangle output counters"),
		flag(tables.Proxy, "Rules proxy exist"), counted(tables.ProxyCounted, "Rules proxy counters"))
	if len(tables.ForeignMarks) > 0 {
		add("⚠️  Additional marking rules found:")
		for _, chain := range tables.ForeignMarks {
			add("\tchain " + chain + " sets a packet mark")
		}
	} else {
		add("✅ Additional marking rules found")
	}

	add(rule, "📄 Podkop config")
	add(strings.TrimSuffix(diag.MaskSettings(settingsContent), "\n"))

	add(rule, "📄 WAN config")
	networkContent, _ := os.ReadFile(paths.Network)
	network, _ := uci.Parse(networkContent)
	if network != nil && network.Section("wan") != nil {
		add(wanBlock(string(networkContent))...)
	} else {
		add("❌ WAN configuration not found")
	}
	if network != nil {
		for _, section := range network.Sections {
			host := strings.TrimSpace(section.Get("endpoint_host"))
			if host == "" {
				continue
			}
			if host == "engage.cloudflareclient.com" {
				add("⚠️ WARP detected: " + host)
				continue
			}
			octets := strings.SplitN(host, ".", 3)
			if len(octets) < 2 {
				continue
			}
			for _, prefix := range warpPrefixes {
				if octets[0]+"."+octets[1] == prefix {
					add(rule, "⚠️ WARP detected: "+host)
				}
			}
		}
		for _, section := range network.Sections {
			if strings.HasPrefix(section.Type, "wireguard_") && section.Get("route_allowed_ips") == "1" && section.Get("allowed_ips") == "0.0.0.0/0" {
				add(rule, "⚠️ WG Route allowed IP enabled with 0.0.0.0/0")
			}
		}
	}
	if _, err := os.Stat("/etc/init.d/zapret"); err == nil {
		add(rule, "⚠️ Zapret detected")
	}

	add(rule, "🥸 FakeIP status")
	if !answered {
		add("❌ Failed to get FakeIP info")
	} else {
		add(mark(works, "✅ Router DNS is routed through sing-box", "⚠️ Router DNS is NOT routed through sing-box"))
	}
	parsed, err := netip.ParseAddr(address)
	add(mark(err == nil && dataplane.FakeIPPrefix.Contains(parsed), "✅ Sing-box works with FakeIP: ", "❌ Sing-box does NOT work with FakeIP: ") + address)
	return strings.Join(lines, "\n") + "\n"
}

// blocks returns the lines of the sections of a settings file that a test of the "config"
// line picks, as they are written.
func blocks(content string, pick func(fields []string) bool) []string {
	var result []string
	inside := false
	for _, line := range strings.Split(strings.TrimSuffix(content, "\n"), "\n") {
		if strings.HasPrefix(line, "config ") {
			fields := strings.Fields(line)
			inside = len(fields) > 1 && pick(fields)
		}
		if inside {
			result = append(result, line)
		}
	}
	return result
}

// wanBlock returns the section of the interface "wan" with its addresses and its
// credentials hidden.
func wanBlock(content string) []string {
	var result []string
	inside := false
	protocol := ""
	for _, line := range strings.Split(strings.TrimSuffix(content, "\n"), "\n") {
		fields := strings.Fields(line)
		if strings.HasPrefix(line, "config ") {
			inside = len(fields) > 2 && fields[1] == "interface" && fields[2] == "'wan'"
			protocol = ""
		}
		if !inside {
			continue
		}
		isOption := len(fields) > 2 && fields[0] == "option"
		switch {
		case isOption && fields[1] == "proto":
			protocol = fields[2]
			result = append(result, line)
		case isOption && protocol == "'static'" && (fields[1] == "ipaddr" || fields[1] == "netmask" || fields[1] == "gateway"),
			isOption && protocol == "'pppoe'" && (fields[1] == "username" || fields[1] == "password"):
			result = append(result, "        option "+fields[1]+" '******'")
		default:
			result = append(result, line)
		}
	}
	return result
}
