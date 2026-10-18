// Package clock sets the clock of the router before the backend starts, as the shell backend
// did with "ntpd -q". Most routers have no clock of their own: one boots with the time of
// the last change of /etc, which may be days or months behind, and TLS refuses certificates
// against such a clock: DoH and DoT, the downloads of lists, the proxies over TLS. The
// router's own NTP service finds its servers by name, and with DoH or DoT the names need
// the very TLS the clock breaks. The servers here are asked by address.
package clock

import (
	"context"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/ntp"
)

// Servers are the NTP servers the shell backend asked, by address.
var Servers = []string{"194.190.168.1", "216.239.35.0", "216.239.35.4", "162.159.200.1", "162.159.200.123"}

// Threshold is how far off the clock may be before it is set: closer than this it is the
// business of the router's NTP service, which adjusts it smoothly.
const Threshold = time.Minute

// Result is what the check came to.
type Result struct {
	// Server answered first.
	Server string
	// Offset is how far the clock was behind (positive) or ahead (negative).
	Offset time.Duration
	// Set tells whether the clock was set.
	Set bool
}

// Sync asks every server at once and takes the first valid answer. A clock off by more than
// threshold is set with set (ntp.SetSystemTime when nil). Without a valid answer before the
// context ends nothing is changed and the error says why.
func Sync(ctx context.Context, servers []string, threshold time.Duration, set func(time.Time) error) (Result, error) {
	if set == nil {
		set = ntp.SetSystemTime
	}
	type answer struct {
		server   string
		response *ntp.Response
		err      error
	}
	answers := make(chan answer, len(servers))
	for _, server := range servers {
		go func(server string) {
			address := M.ParseSocksaddr(server)
			if address.Port == 0 {
				address.Port = 123
			}
			response, err := ntp.Exchange(ctx, N.SystemDialer, address)
			if err == nil {
				err = response.Validate()
			}
			answers <- answer{server, response, err}
		}(server)
	}
	var problems []error
	for range servers {
		select {
		case <-ctx.Done():
			return Result{}, E.Cause(ctx.Err(), "no NTP server answered (", len(problems), " failed)")
		case got := <-answers:
			if got.err != nil {
				problems = append(problems, E.Cause(got.err, got.server))
				continue
			}
			result := Result{Server: got.server, Offset: got.response.ClockOffset}
			if result.Offset > threshold || result.Offset < -threshold {
				err := set(time.Now().Add(result.Offset))
				if err != nil {
					return result, E.Cause(err, "set the clock")
				}
				result.Set = true
			}
			return result, nil
		}
	}
	return Result{}, E.Errors(problems...)
}
