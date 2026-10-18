package engine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// A link is decoded without sing-box building its outbound; CheckLink builds it and tells why
// this sing-box cannot use it, without what the link keeps secret.
func TestDecodeLinkDoesNotBuild(t *testing.T) {
	const canary = "CANARY0f6d2c"
	const uuid = "6f1c9a0e-3b52-4c1e-9a55-2d4a1b0f7c11"
	// a REALITY key that is none: the decoder takes it, the outbound cannot be built
	link := "vless://" + uuid + "@example.org:443?security=reality&pbk=" + canary + "&sid=01"
	_, _, err := DecodeLink(link)
	require.NoError(t, err)
	err = CheckLink(link)
	require.Error(t, err)
	require.NotContains(t, err.Error(), canary)
	require.NotContains(t, err.Error(), uuid)
	require.NoError(t, CheckLink("socks5://203.0.113.15:1080"))
	// what the decoder refuses is refused by both
	_, _, err = DecodeLink("vless://broken")
	require.Error(t, err)
	require.Error(t, CheckLink("vless://broken"))
}
