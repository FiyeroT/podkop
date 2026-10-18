package clock

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// an NTP server on this host that tells the time now plus skew
func fakeServer(t *testing.T, skew time.Duration) string {
	connection, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { connection.Close() })
	go func() {
		request := make([]byte, 48)
		for {
			_, peer, err := connection.ReadFrom(request)
			if err != nil {
				return
			}
			stamp := ntpTime(time.Now().Add(skew))
			answer := make([]byte, 48)
			answer[0] = 0<<6 | 4<<3 | 4                    // in sync, version 4, server
			answer[1] = 2                                  // stratum
			binary.BigEndian.PutUint64(answer[16:], stamp) // reference
			copy(answer[24:32], request[40:48])            // origin: the client's transmit
			binary.BigEndian.PutUint64(answer[32:], stamp) // receive
			binary.BigEndian.PutUint64(answer[40:], stamp) // transmit
			connection.WriteTo(answer, peer)
		}
	}()
	return connection.LocalAddr().String()
}

func ntpTime(t time.Time) uint64 {
	const epoch = 2208988800 // seconds from 1900 to 1970
	seconds := uint64(t.Unix()) + epoch
	fraction := uint64(t.Nanosecond()) << 32 / 1e9
	return seconds<<32 | fraction
}

func TestSync(t *testing.T) {
	var setTo time.Time
	set := func(now time.Time) error { setTo = now; return nil }
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// a clock two hours behind is set
	behind := fakeServer(t, 2*time.Hour)
	result, err := Sync(ctx, []string{behind}, Threshold, set)
	require.NoError(t, err)
	require.True(t, result.Set)
	require.InDelta(t, float64(2*time.Hour), float64(result.Offset), float64(time.Second))
	require.WithinDuration(t, time.Now().Add(2*time.Hour), setTo, 2*time.Second)

	// a few seconds off is left to the router's NTP service
	setTo = time.Time{}
	near := fakeServer(t, 5*time.Second)
	result, err = Sync(ctx, []string{near}, Threshold, set)
	require.NoError(t, err)
	require.False(t, result.Set)
	require.True(t, setTo.IsZero())

	// a server that does not answer does not hold the others back
	silent, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	defer silent.Close()
	started := time.Now()
	result, err = Sync(ctx, []string{silent.LocalAddr().String(), behind}, Threshold, set)
	require.NoError(t, err)
	require.Equal(t, behind, result.Server)
	require.Less(t, time.Since(started), time.Second)

	// none answers: an error when the time is up, and the clock untouched
	setTo = time.Time{}
	short, cancelShort := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancelShort()
	_, err = Sync(short, []string{silent.LocalAddr().String()}, Threshold, set)
	require.Error(t, err)
	require.True(t, setTo.IsZero())
}
