package repeater

import (
	"net"
	"testing"
	"time"
)

func TestNewRepeater(t *testing.T) {
	callsign := "W1ABC"
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 42000}

	rep := NewRepeater(callsign, addr)

	if rep.Callsign() != callsign {
		t.Errorf("Expected callsign %s, got %s", callsign, rep.Callsign())
	}

	if rep.Address().String() != addr.String() {
		t.Errorf("Expected address %s, got %s", addr.String(), rep.Address().String())
	}

	if !rep.IsActive() {
		t.Errorf("Expected new repeater to be active")
	}

	if rep.IsTalking() {
		t.Errorf("Expected new repeater to not be talking")
	}

	if rep.PacketCount() != 0 {
		t.Errorf("Expected packet count to be 0, got %d", rep.PacketCount())
	}
}

func TestRepeaterCounters(t *testing.T) {
	callsign := "W1ABC"
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 42000}

	rep := NewRepeater(callsign, addr)

	// Test packet counter
	rep.IncrementPacketCount()
	rep.IncrementPacketCount()
	if rep.PacketCount() != 2 {
		t.Errorf("Expected packet count 2, got %d", rep.PacketCount())
	}

	// Test bytes received
	rep.AddBytesReceived(100)
	rep.AddBytesReceived(50)
	if rep.BytesReceived() != 150 {
		t.Errorf("Expected bytes received 150, got %d", rep.BytesReceived())
	}

	// Test bytes transmitted
	rep.AddBytesTransmitted(200)
	if rep.BytesTransmitted() != 200 {
		t.Errorf("Expected bytes transmitted 200, got %d", rep.BytesTransmitted())
	}
}

func TestRepeaterTalkState(t *testing.T) {
	callsign := "W1ABC"
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 42000}

	rep := NewRepeater(callsign, addr)

	// Initially not talking
	if rep.IsTalking() {
		t.Errorf("Expected repeater to not be talking initially")
	}

	if rep.TalkDuration() != 0 {
		t.Errorf("Expected talk duration to be 0 when not talking")
	}

	// Start talking
	rep.StartTalking()
	if !rep.IsTalking() {
		t.Errorf("Expected repeater to be talking after StartTalking()")
	}

	// Wait a bit and check duration
	time.Sleep(10 * time.Millisecond)
	duration := rep.TalkDuration()
	if duration < 10*time.Millisecond {
		t.Errorf("Expected talk duration to be at least 10ms, got %v", duration)
	}

	// Stop talking
	stopDuration := rep.StopTalking()
	if rep.IsTalking() {
		t.Errorf("Expected repeater to not be talking after StopTalking()")
	}

	if stopDuration < 10*time.Millisecond {
		t.Errorf("Expected stop duration to be at least 10ms, got %v", stopDuration)
	}

	// Talk duration should be 0 again
	if rep.TalkDuration() != 0 {
		t.Errorf("Expected talk duration to be 0 after stopping")
	}
}

func TestRepeaterTimeout(t *testing.T) {
	callsign := "W1ABC"
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 42000}

	rep := NewRepeater(callsign, addr)

	// Should not be timed out immediately
	if rep.IsTimedOut(1 * time.Minute) {
		t.Errorf("Expected repeater to not be timed out immediately")
	}

	// Should be timed out with very short timeout
	if !rep.IsTimedOut(1 * time.Nanosecond) {
		t.Errorf("Expected repeater to be timed out with 1ns timeout")
	}

	// Update last seen and check again
	rep.UpdateLastSeen()
	if rep.IsTimedOut(1 * time.Minute) {
		t.Errorf("Expected repeater to not be timed out after updating last seen")
	}
}

func TestRepeaterStats(t *testing.T) {
	callsign := "W1ABC"
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 42000}

	rep := NewRepeater(callsign, addr)
	rep.IncrementPacketCount()
	rep.AddBytesReceived(100)
	rep.StartTalking()

	stats := rep.Stats()

	if stats.Callsign != callsign {
		t.Errorf("Expected callsign %s in stats, got %s", callsign, stats.Callsign)
	}

	// Address is masked for privacy (last two octets replaced with **)
	expectedMasked := "127.0.**:42000"
	if stats.Address != expectedMasked {
		t.Errorf("Expected masked address %s in stats, got %s", expectedMasked, stats.Address)
	}

	if stats.PacketCount != 1 {
		t.Errorf("Expected packet count 1 in stats, got %d", stats.PacketCount)
	}

	if stats.BytesReceived != 100 {
		t.Errorf("Expected bytes received 100 in stats, got %d", stats.BytesReceived)
	}

	if !stats.IsTalking {
		t.Errorf("Expected IsTalking to be true in stats")
	}

	if !stats.IsActive {
		t.Errorf("Expected IsActive to be true in stats")
	}
}

func TestRepeaterString(t *testing.T) {
	callsign := "W1ABC"
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 42000}

	rep := NewRepeater(callsign, addr)
	str := rep.String()

	if str == "" {
		t.Errorf("Expected non-empty string representation")
	}

	// Should contain callsign
	if !containsString(str, callsign) {
		t.Errorf("String representation should contain callsign")
	}

	// Test talking state
	rep.StartTalking()
	talkingStr := rep.String()
	if !containsString(talkingStr, "talking") {
		t.Errorf("String representation should indicate talking state")
	}

	// Test inactive state
	rep.SetActive(false)
	inactiveStr := rep.String()
	if !containsString(inactiveStr, "inactive") {
		t.Errorf("String representation should indicate inactive state")
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
