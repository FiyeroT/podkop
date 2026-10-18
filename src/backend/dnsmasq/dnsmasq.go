// Package dnsmasq points the DNS of the LAN at the backend and back. dnsmasq stays the
// resolver the devices ask; its upstream becomes the DNS inbound of sing-box, and what it
// had before is kept in its own settings under other names, as the shell backend keeps
// it, so that either backend can undo what the other did.
package dnsmasq

import (
	"os"
	"os/exec"
	"strings"

	"github.com/itdoginfo/podkop/src/backend/dataplane"
	"github.com/itdoginfo/podkop/src/backend/uci"
	E "github.com/sagernet/sing/common/exceptions"
)

const (
	// address is where the DNS inbound of the backend listens
	address = dataplane.DNSAddress
	// what dnsmasq falls back to when nothing was kept: the resolvers netifd collects
	resolvFile = "/tmp/resolv.conf.d/resolv.conf.auto"
	section    = "dhcp.@dnsmasq[0]"
	// marker is the option the backend leaves on a dnsmasq it took over: what Restore
	// gives back is the backend's own doing, never a setting made by hand
	marker = "podkop_taken"
)

// System is the router as this package touches it; a test has its own.
type System interface {
	// ReadSettings returns the UCI file of dnsmasq.
	ReadSettings() ([]byte, error)
	// Batch runs commands of the uci program, one a line.
	Batch(script string) error
	// Restart restarts dnsmasq.
	Restart() error
	// Exists tells whether a file is there.
	Exists(path string) bool
}

// Router is the System of an OpenWrt router.
type Router struct{}

func (Router) ReadSettings() ([]byte, error) { return os.ReadFile("/etc/config/dhcp") }

func (Router) Batch(script string) error {
	command := exec.Command("uci", "-q", "batch")
	command.Stdin = strings.NewReader(script)
	output, err := command.CombinedOutput()
	if err != nil {
		return E.Cause(err, "uci batch: ", strings.TrimSpace(string(output)))
	}
	return nil
}

func (Router) Restart() error {
	output, err := exec.Command("/etc/init.d/dnsmasq", "restart").CombinedOutput()
	if err != nil {
		return E.Cause(err, "restart dnsmasq: ", strings.TrimSpace(string(output)))
	}
	return nil
}

func (Router) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// state is what the first dnsmasq of the settings has.
type state struct {
	servers   []string
	noResolv  string
	cacheSize string
	// what was kept of the settings before the backend took dnsmasq over
	keptServers   []string
	keptNoResolv  string
	keptCacheSize string
	// marked tells that the backend took it over
	marked bool
}

func read(system System) (*state, error) {
	content, err := system.ReadSettings()
	if err != nil {
		return nil, E.Cause(err, "read the settings of dnsmasq")
	}
	parsed, err := uci.Parse(content)
	if err != nil {
		return nil, E.Cause(err, "read the settings of dnsmasq")
	}
	sections := parsed.SectionsOfType("dnsmasq")
	if len(sections) == 0 {
		return nil, E.New("the settings have no dnsmasq")
	}
	first := sections[0]
	return &state{
		servers:   strings.Fields(first.Get("server")),
		noResolv:  first.Get("noresolv"),
		cacheSize: first.Get("cachesize"),
		// compat: dnsmasq-backup-keys
		keptServers:   strings.Fields(first.Get("podkop_server")),
		keptNoResolv:  first.Get("podkop_noresolv"),
		keptCacheSize: first.Get("podkop_cachesize"),
		marked:        first.Get(marker) == "1",
	}, nil
}

// taken tells whether a backend took this dnsmasq over: this one leaves its marker, the
// shell backend and earlier releases of this one leave what they kept (the cache size,
// which OpenWrt sets by default, among it).
func (s *state) taken() bool {
	return s.marked || len(s.keptServers) > 0 || s.keptNoResolv != "" || s.keptCacheSize != ""
}

// ours tells whether dnsmasq asks the backend and nothing else, with no cache of its own.
func (s *state) ours() bool {
	return len(s.servers) == 1 && s.servers[0] == address && s.noResolv == "1" && s.cacheSize == "0"
}

// Ours tells whether dnsmasq is set up to ask the backend.
func Ours(system System) (bool, error) {
	current, err := read(system)
	if err != nil {
		return false, err
	}
	return current.ours(), nil
}

// TakeOver makes dnsmasq ask the backend: its servers, its "noresolv" and its cache size
// are kept under other names, and it is restarted. A dnsmasq that asks the backend
// already is left alone, without a restart: the shell backend restarts it on every start,
// which takes the DNS of the LAN away for three seconds each time.
// It tells whether anything was changed.
func TakeOver(system System) (bool, error) {
	current, err := read(system)
	if err != nil {
		return false, err
	}
	if current.ours() {
		return false, nil
	}
	var script strings.Builder
	hadServers := false
	for _, server := range current.servers {
		hadServers = true
		if server != address {
			script.WriteString("add_list " + section + ".podkop_server=" + quote(server) + "\n")
		}
	}
	if hadServers {
		script.WriteString("delete " + section + ".server\n")
	}
	if current.noResolv != "" {
		script.WriteString("set " + section + ".podkop_noresolv=" + quote(current.noResolv) + "\n")
	}
	if current.cacheSize != "" {
		script.WriteString("set " + section + ".podkop_cachesize=" + quote(current.cacheSize) + "\n")
	}
	script.WriteString("add_list " + section + ".server=" + quote(address) + "\n")
	script.WriteString("set " + section + ".noresolv='1'\n")
	script.WriteString("set " + section + ".cachesize='0'\n")
	script.WriteString("set " + section + "." + marker + "='1'\n")
	script.WriteString("commit dhcp\n")
	err = system.Batch(script.String())
	if err != nil {
		return false, err
	}
	return true, system.Restart()
}

// Restore gives dnsmasq back what was kept when it was taken over, and restarts it. With
// nothing kept it gets the resolvers netifd collects, as the shell backend does it. Only
// a dnsmasq a backend took over is given back (see taken): one that was set to ask the
// backend by hand, with dont_touch_dhcp, is left as it is. The shell backend instead
// leaves dnsmasq alone whenever dont_touch_dhcp is set, also when it had taken it over
// before the option was turned on, and the LAN is then without DNS after the stop.
// It tells whether anything was changed.
func Restore(system System) (bool, error) {
	current, err := read(system)
	if err != nil {
		return false, err
	}
	if !current.taken() {
		return false, nil
	}
	var script strings.Builder
	if current.marked {
		script.WriteString("delete " + section + "." + marker + "\n")
	}
	if current.keptCacheSize == "" {
		script.WriteString("set " + section + ".cachesize='150'\n")
	} else {
		script.WriteString("set " + section + ".cachesize=" + quote(current.keptCacheSize) + "\n")
		script.WriteString("delete " + section + ".podkop_cachesize\n")
	}
	if current.keptNoResolv == "" {
		script.WriteString("set " + section + ".noresolv='0'\n")
	} else {
		script.WriteString("set " + section + ".noresolv=" + quote(current.keptNoResolv) + "\n")
		script.WriteString("delete " + section + ".podkop_noresolv\n")
	}
	if len(current.servers) > 0 {
		script.WriteString("delete " + section + ".server\n")
	}
	if len(current.keptServers) > 0 {
		for _, server := range current.keptServers {
			script.WriteString("add_list " + section + ".server=" + quote(server) + "\n")
		}
		script.WriteString("delete " + section + ".podkop_server\n")
	} else if system.Exists(resolvFile) {
		script.WriteString("set " + section + ".resolvfile=" + quote(resolvFile) + "\n")
		if isOne(current.keptNoResolv) {
			// no servers to ask and told not to read the resolvers: it would ask nobody
			script.WriteString("set " + section + ".noresolv='0'\n")
		}
	}
	script.WriteString("commit dhcp\n")
	err = system.Batch(script.String())
	if err != nil {
		return false, err
	}
	return true, system.Restart()
}

func isOne(text string) bool {
	text = strings.TrimLeft(strings.TrimSpace(text), "+0")
	return text == "1"
}

// quote writes a value for the uci program.
func quote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
