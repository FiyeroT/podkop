package engine

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/stretchr/testify/require"
)

// A hook is told what sing-box knows of a routed connection, and the connection passes
// as it is.
func TestHookTracker(t *testing.T) {
	var told []Connection
	tracker := hookTracker{func(connection Connection) { told = append(told, connection) }}
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	// through the transparent proxy with a FakeIP address, once a rule has sent the
	// connection to another address
	require.Equal(t, left, tracker.RoutedConnection(context.Background(), left, adapter.InboundContext{
		Inbound: "tproxy-in", Source: M.ParseSocksaddr("[::ffff:192.168.1.23]:50000"), FakeIP: true,
		OriginDestination:        M.ParseSocksaddr("198.18.0.7:443"),
		RouteOriginalDestination: M.Socksaddr{Fqdn: "replaced.example", Port: 443},
		Destination:              M.ParseSocksaddr("127.0.0.1:1603"),
		Domain:                   "sniffed.example",
	}, nil, nil))
	// a packet connection to a name, without FakeIP
	require.Nil(t, tracker.RoutedPacketConnection(context.Background(), nil, adapter.InboundContext{
		Inbound: "mixed-in", Source: M.ParseSocksaddr("192.168.1.24:50001"),
		OriginDestination: M.ParseSocksaddr("203.0.113.1:443"),
		Destination:       M.Socksaddr{Fqdn: "named.example", Port: 443},
	}, nil, nil))
	require.Nil(t, tracker.RoutedFlow(context.Background(), adapter.InboundContext{}, nil, nil))

	require.Equal(t, []Connection{
		{
			Inbound: "tproxy-in", Source: netip.MustParseAddr("192.168.1.23"),
			ReplacedName: "replaced.example", SniffedName: "sniffed.example",
			FakeIP: true, Address: netip.MustParseAddr("198.18.0.7"),
		},
		{Inbound: "mixed-in", Source: netip.MustParseAddr("192.168.1.24"), Name: "named.example"},
	}, told)
}
