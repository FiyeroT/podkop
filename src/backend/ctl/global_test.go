package ctl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWANBlock(t *testing.T) {
	t.Parallel()
	network := `config interface 'lan'
	option proto 'static'
	option ipaddr '192.168.1.1'

config interface 'wan'
	option device 'eth1'
	option proto 'pppoe'
	option username 'me'
	option password 'secret'
	option ipaddr '10.0.0.2'

config interface 'wan6'
	option proto 'dhcpv6'
`
	require.Equal(t, []string{"config interface 'wan'", "\toption device 'eth1'", "\toption proto 'pppoe'", "        option username '******'", "        option password '******'", "\toption ipaddr '10.0.0.2'", ""}, wanBlock(network))
	static := "config interface 'wan'\n\toption proto 'static'\n\toption ipaddr '198.51.100.2'\n\toption netmask '255.255.255.0'\n\toption gateway '198.51.100.1'\n\toption dns '1.1.1.1'\n"
	require.Equal(t, []string{"config interface 'wan'", "\toption proto 'static'", "        option ipaddr '******'", "        option netmask '******'", "        option gateway '******'", "\toption dns '1.1.1.1'"}, wanBlock(static))
	require.Equal(t, []string{"config dnsmasq", "\toption noresolv '1'"}, blocks("config dnsmasq\n\toption noresolv '1'\nconfig dhcp 'lan'\n\toption start '100'\n", func(fields []string) bool { return fields[1] == "dnsmasq" }))
}
