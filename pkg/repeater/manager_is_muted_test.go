package repeater

import (
	"net"
	"testing"
	"time"
)

func TestIsMuted(t *testing.T) {
	m := NewManager(5*time.Minute, 100, nil, 10*time.Second, 30*time.Second)
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:50000")

	t.Run("unmuted", func(t *testing.T) {
		if m.IsMuted(addr) {
			t.Fatal("expected not muted")
		}
	})

	t.Run("muted-zero", func(t *testing.T) {
		// zero time -> muted until they stop
		m.muted.Store(addr.String(), time.Time{})
		if !m.IsMuted(addr) {
			t.Fatal("expected muted for zero time")
		}
		m.muted.Delete(addr.String())
	})

	t.Run("muted-future", func(t *testing.T) {
		m.muted.Store(addr.String(), time.Now().Add(1*time.Hour))
		if !m.IsMuted(addr) {
			t.Fatal("expected muted for future unmute time")
		}
		m.muted.Delete(addr.String())
	})

	t.Run("muted-expired", func(t *testing.T) {
		m.muted.Store(addr.String(), time.Now().Add(-1*time.Hour))
		if m.IsMuted(addr) {
			t.Fatal("expected not muted for expired unmute time")
		}
		// cleanup
		m.muted.Delete(addr.String())
	})
}
