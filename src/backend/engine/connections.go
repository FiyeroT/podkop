package engine

import (
	"context"
	"net"
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-tun"
	N "github.com/sagernet/sing/common/network"
)

// Connection is what sing-box knows of a connection it routes, as a hook is told it.
type Connection struct {
	// Inbound is the tag of the inbound the connection came in through.
	Inbound string
	// Source is who opened it.
	Source netip.Addr
	// The names of its destination, any of them may be empty: the one a rule replaced by
	// an address, the one the address is of, the one sniffed from the first bytes.
	ReplacedName string
	Name         string
	SniffedName  string
	// FakeIP tells that the connection was opened to a FakeIP address of the name,
	// Address which one.
	FakeIP  bool
	Address netip.Addr
}

// ConnectionHook is told of every connection an instance routes, by the goroutine of the
// connection after the route rules matched, before the connection goes on. It must not
// block, and it is on the way of every connection: it should not allocate either.
type ConnectionHook func(connection Connection)

// hookTracker is a connection tracker of sing-box that tells a hook of each connection
// and lets it through as it is. A connection a rule rejected is not routed and not told.
type hookTracker struct {
	hook ConnectionHook
}

func (t hookTracker) tell(metadata *adapter.InboundContext) {
	connection := Connection{
		Inbound:      metadata.Inbound,
		Source:       metadata.Source.Addr.Unmap(),
		ReplacedName: metadata.RouteOriginalDestination.Fqdn,
		Name:         metadata.Destination.Fqdn,
		SniffedName:  metadata.Domain,
		FakeIP:       metadata.FakeIP,
	}
	if metadata.FakeIP {
		connection.Address = metadata.OriginDestination.Addr.Unmap()
	}
	t.hook(connection)
}

func (t hookTracker) RoutedConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) net.Conn {
	t.tell(&metadata)
	return conn
}

func (t hookTracker) RoutedPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) N.PacketConn {
	t.tell(&metadata)
	return conn
}

func (t hookTracker) RoutedFlow(ctx context.Context, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) tun.FlowTracker {
	return nil
}
