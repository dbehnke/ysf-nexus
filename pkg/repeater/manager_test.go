package repeater

import (
    "context"
    "net"
    "testing"
    "time"
)

// helper to create UDP address
func mustAddr(t *testing.T, s string) *net.UDPAddr {
    t.Helper()
    a, err := net.ResolveUDPAddr("udp", s)
    if err != nil {
        t.Fatalf("resolve addr: %v", err)
    }
    return a
}

func TestSingleActiveStreamEnforcement(t *testing.T) {
    events := make(chan Event, 10)
    m := NewManager(5*time.Second, 10, events, 180*time.Second, 0)

    addr1 := mustAddr(t, "127.0.0.1:40001")
    addr2 := mustAddr(t, "127.0.0.1:40002")

    r1, _ := m.AddRepeater("R1", addr1)
    r2, _ := m.AddRepeater("R2", addr2)

    // First repeater starts talking
    m.ProcessPacket(r1.Callsign(), addr1, "YSFD", 100)
    if !r1.IsTalking() {
        t.Fatalf("expected r1 to be talking")
    }

    // Second repeater attempts to talk but should be ignored
    m.ProcessPacket(r2.Callsign(), addr2, "YSFD", 100)
    if r2.IsTalking() {
        t.Fatalf("expected r2 NOT to be talking while r1 is active")
    }

    // Stop r1 by directly stopping talking (manager will clear activeKey when it observes the stop via timeouts).
    _ = r1.StopTalking()

    // For unit test determinism, clear activeKey to simulate manager recognizing the stop
    m.activeMu.Lock()
    m.activeKey = ""
    m.activeMu.Unlock()

    // Now r2 should be able to start
    m.ProcessPacket(r2.Callsign(), addr2, "YSFD", 50)
    if !r2.IsTalking() {
        t.Fatalf("expected r2 to start talking after r1 stopped")
    }
}

func TestMuteOnTalkMaxDuration(t *testing.T) {
    events := make(chan Event, 10)
    m := NewManager(5*time.Second, 10, events, 100*time.Millisecond, 0)

    addr1 := mustAddr(t, "127.0.0.1:41001")
    r1, _ := m.AddRepeater("R3", addr1)

    // Start talking
    m.ProcessPacket(r1.Callsign(), addr1, "YSFD", 100)
    if !r1.IsTalking() {
        t.Fatalf("expected r1 to be talking")
    }

    // Keep updating talk data briefly to simulate continuous talk
    time.Sleep(150 * time.Millisecond)
    // Call ProcessPacket again as if more data arrived; this should trigger mute logic
    m.ProcessPacket(r1.Callsign(), addr1, "YSFD", 50)

    // r1 should have been muted and stopped talking
    if r1.IsTalking() {
        t.Fatalf("expected r1 to be stopped after exceeding talkMaxDuration")
    }

    // Muted entry should exist
    if _, ok := m.muted.Load(addr1.String()); !ok {
        t.Fatalf("expected r1 to be muted after exceeding talkMaxDuration")
    }

    // Clean up goroutine context to ensure no background work
    ctx, cancel := context.WithCancel(context.Background())
    cancel()
    _ = ctx
}
