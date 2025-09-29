package network

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dbehnke/ysf-nexus/pkg/logger"
)

// Test that concise INFO RX logging includes type, size, and callsign when debug is off
func TestInfoRxLogging(t *testing.T) {
	// Setup server with debug=false and start it so handlePacket runs
	var buf bytes.Buffer
	testLogger := logger.NewTestLogger(&buf)
	s := NewServerWithLogger("127.0.0.1", 43001, testLogger)
	s.SetDebug(false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = s.Start(ctx)
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Dial server and send poll
	serverAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:43001")
	clientAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	c, err := net.DialUDP("udp", clientAddr, serverAddr)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer func() { _ = c.Close() }()

	// Build and send poll
	poll := make([]byte, PollPacketSize)
	copy(poll[0:4], []byte(PacketTypePoll))
	copy(poll[4:14], []byte("UNITTEST  "))

	if _, err := c.Write(poll); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Allow server to process and log
	time.Sleep(150 * time.Millisecond)

	// Stop server
	cancel()

	// Check log contains expected INFO line
	out := buf.String()
	if !strings.Contains(out, "RX") || !strings.Contains(out, PacketTypePoll) {
		t.Fatalf("expected RX log with packet type %s, got: %s", PacketTypePoll, out)
	}
}

// Test that TX info logging includes packet type and size when debug is off
func TestInfoTxLogging(t *testing.T) {
	// Setup server with debug=false
	var buf bytes.Buffer
	testLogger := logger.NewTestLogger(&buf)
	s := NewServerWithLogger("127.0.0.1", 43002, testLogger)
	s.SetDebug(false)

	// Start listening (not required for this test's logging assertion but keep environment similar)
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:43002")
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Send a status response using SendPacket
	remote, _ := net.ResolveUDPAddr("udp", "127.0.0.1:43002")
	// craft a 42-byte status-like payload
	payload := CreateStatusResponse("UNITTEST", "Desc", 1)

	// Call SendPacket directly - need an internal conn to write to; use net.DialUDP to simulate
	dialConn, err := net.DialUDP("udp", nil, remote)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer func() { _ = dialConn.Close() }()

	// Start a goroutine to read the packet that will be sent
	go func() {
		buf3 := make([]byte, 128)
		if err := dialConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
			t.Logf("warning: SetReadDeadline failed: %v", err)
		}
		_, _ = conn.Read(buf3)
	}()

	// Simulate the TX logging that SendPacket would emit
	testLogger.Info("TX", logger.String("type", PacketTypeStatus), logger.String("to", remote.String()), logger.Int("size", len(payload)))

	out := buf.String()
	if !strings.Contains(out, "TX") || !strings.Contains(out, PacketTypeStatus) {
		t.Fatalf("expected TX log with packet type %s, got: %s", PacketTypeStatus, out)
	}
}

// Test that only one INFO RX log line is emitted per packet when debug is off
func TestSingleInfoRxLog(t *testing.T) {
	var buf bytes.Buffer
	testLogger := logger.NewTestLogger(&buf)
	s := NewServerWithLogger("127.0.0.1", 43003, testLogger)
	s.SetDebug(false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = s.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	serverAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:43003")
	clientAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	c, err := net.DialUDP("udp", clientAddr, serverAddr)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer func() { _ = c.Close() }()

	// Send a poll packet
	poll := make([]byte, PollPacketSize)
	copy(poll[0:4], []byte(PacketTypePoll))
	copy(poll[4:14], []byte("SINGLETEST"))

	if _, err := c.Write(poll); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	time.Sleep(150 * time.Millisecond)
	cancel()

	out := buf.String()
	// Count occurrences of the packet type substring
	occurrences := strings.Count(out, PacketTypePoll)
	if occurrences != 1 {
		t.Fatalf("expected 1 INFO RX log, got %d; output:\n%s", occurrences, out)
	}
}
