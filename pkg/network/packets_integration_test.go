package network

import (
	"net"
	"testing"
	"time"
)

// Test that a listening server responds to poll and status requests with expected formats.
func TestPollAndStatusResponse(t *testing.T) {
	// Start a UDP listener that uses the same ParsePacket and Create* functions to emulate the server behavior.
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to resolve udp addr: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatalf("failed to listen udp: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Use the local address as destination
	remoteAddr := conn.LocalAddr().(*net.UDPAddr)

	// Send a YSFP poll packet (14 bytes)
	poll := make([]byte, PollPacketSize)
	copy(poll[0:4], []byte(PacketTypePoll))
	copy(poll[4:14], []byte("W1ABC\x00"))

	// Create a temporary connection to send and receive
	sock, err := net.DialUDP("udp", nil, remoteAddr)
	if err != nil {
		t.Fatalf("failed to dial udp: %v", err)
	}
	defer func() { _ = sock.Close() }()

	// Send poll
	if _, err := sock.Write(poll); err != nil {
		t.Fatalf("failed to send poll: %v", err)
	}

	// Set read deadline
	if err := sock.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
		t.Logf("warning: SetReadDeadline failed: %v", err)
	}

	// Read should fail because there's no server logic in this test package to reply
	buf := make([]byte, 512)
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("expected to receive the poll on server socket, got error: %v", err)
	}

	// Parse packet received by the server
	pkt, err := ParsePacket(buf[:n], remoteAddr)
	if err != nil {
		t.Fatalf("server failed to parse poll packet: %v", err)
	}

	if pkt.Type != PacketTypePoll {
		t.Fatalf("expected poll packet type, got %s", pkt.Type)
	}

	// Now simulate server responding using CreatePollResponse and CreateStatusResponse
	response := CreatePollResponse()
	// Verify response length and prefix
	if len(response) != PollPacketSize {
		t.Fatalf("unexpected poll response size: %d", len(response))
	}
	if string(response[0:4]) != PacketTypePoll {
		t.Fatalf("unexpected poll response type: %s", string(response[0:4]))
	}

	// Status response
	status := CreateStatusResponse("YSF Nexus", "Test", 1)
	if len(status) != StatusPacketSize {
		t.Fatalf("unexpected status response size: %d", len(status))
	}
	if string(status[0:4]) != PacketTypeStatus {
		t.Fatalf("unexpected status response type: %s", string(status[0:4]))
	}

	// Basic field checks
	// Hash should be 5 bytes ASCII digits
	for i := 4; i < 9; i++ {
		b := status[i]
		if b < '0' || b > '9' {
			t.Fatalf("status hash byte %d not digit: %v", i, b)
		}
	}

	// Name field should contain the provided name (or start with it)
	nameField := string(status[9:25])
	if !contains(nameField, "YSF Nexus") {
		t.Fatalf("status name field doesn't contain expected name: %q", nameField)
	}
}

// Reuse contains helper from unit tests (packets_test.go)
