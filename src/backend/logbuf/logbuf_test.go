package logbuf

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuffer(t *testing.T) {
	t.Parallel()
	buffer := New(50)
	buffer.now = func() time.Time { return time.Date(2026, 10, 12, 20, 0, 0, 0, time.UTC) }
	lines, next := buffer.Since(0)
	require.Empty(t, lines)
	require.Zero(t, next)
	buffer.Write("backend", "info", "running")
	buffer.Write("sing-box", "error", "dns: exchange failed")
	lines, next = buffer.Since(0)
	require.Len(t, lines, 2)
	require.Equal(t, uint64(2), next)
	require.Equal(t, "2026-10-12 20:00:00 podkop: [info] running", lines[0].Text())
	// a line of sing-box does not read as an error of the backend
	require.Equal(t, "2026-10-12 20:00:00 sing-box: error: dns: exchange failed", lines[1].Text())
	require.NotContains(t, lines[1].Text(), "[error]")

	// nothing new since the last line
	lines, next = buffer.Since(next)
	require.Empty(t, lines)
	require.Equal(t, uint64(2), next)

	// more than the buffer holds: the oldest lines go, the numbers go on
	for range 10 {
		buffer.Write("backend", "info", strings.Repeat("x", 20))
	}
	lines, next = buffer.Since(2)
	require.Equal(t, uint64(12), next)
	require.Len(t, lines, 2)
	require.Equal(t, uint64(11), lines[0].Number)
	// a reader that fell behind gets what is left
	lines, _ = buffer.Since(0)
	require.Len(t, lines, 2)

	// one line larger than the buffer is kept alone
	buffer.Write("backend", "warn", strings.Repeat("y", 500))
	lines, _ = buffer.Since(0)
	require.Len(t, lines, 1)
}

func TestBufferFromManyWriters(t *testing.T) {
	t.Parallel()
	buffer := New(1 << 20)
	var writers sync.WaitGroup
	for range 8 {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for range 200 {
				buffer.Write("sing-box", "info", "line")
				buffer.Since(0)
			}
		}()
	}
	writers.Wait()
	lines, next := buffer.Since(0)
	require.Len(t, lines, 1600)
	require.Equal(t, uint64(1600), next)
	for index, line := range lines {
		require.Equal(t, uint64(index+1), line.Number)
	}
}
