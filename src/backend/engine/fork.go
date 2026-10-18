package engine

import (
	"context"
	"net/netip"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns/transport/fakeip"
	"github.com/sagernet/sing-box/route"
	R "github.com/sagernet/sing-box/route/rule"
)

// What the engine uses of the fork of sing-box it is built into besides the API of
// sing-box itself. Each is held to its type here, so that a rebase of the fork that loses
// one does not build: the run-time check of the interface that remains
// fails only for an object that is not the fork's own.
type (
	// peeker is the FakeIP store of the fork: it tells the address of a name without
	// giving one to a name that has none.
	peeker interface {
		Peek(domain string, isIPv6 bool) (netip.Addr, bool)
	}
	// ruleSetUpdater is the router of the fork: it downloads the remote rule sets now.
	ruleSetUpdater interface {
		UpdateRuleSets(ctx context.Context) (R.RuleSetUpdateSummary, error)
	}
)

var (
	_ peeker         = (*fakeip.Store)(nil)
	_ ruleSetUpdater = (*route.Router)(nil)
)

// StopTimeout is how long an instance that is closed is waited for as a rule: sing-box's
// own limit for its close.
const StopTimeout = C.FatalStopTimeout
