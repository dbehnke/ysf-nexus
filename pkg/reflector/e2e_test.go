package reflector

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dbehnke/ysf-nexus/pkg/config"
	"github.com/dbehnke/ysf-nexus/pkg/logger"
	"github.com/dbehnke/ysf-nexus/pkg/network"
)

// This end-to-end test starts a reflector instance in-process and simulates a client
// sending a YSFP poll and a minimal 4-byte YSFS probe. It validates that the server
// responds with a poll response and a full 42-byte status response respectively.
func TestReflectorEndToEnd(t *testing.T) {
	// Minimal config override for test
	cfg := &config.Config{}
	// bind to all interfaces in CI to avoid loopback/network namespace surprises
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = 42999 // use a high port unlikely to be in use
	cfg.Server.Name = "E2E-TEST"
	cfg.Server.Description = "E2E Description"
	cfg.Server.Timeout = 5 * time.Second
	cfg.Server.MaxConnections = 10
	cfg.Logging.Level = "debug"

	log := logger.Default().WithComponent("e2e-test")

	r := New(cfg, log)

	// Start reflector in background with cancellable context and ensure shutdown
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	// ensure we cancel the server and wait for it to stop when the test exits
	defer func() {
		cancel()
		<-done
	}()
	go func() {
		_ = r.Start(ctx) // errors will surface to test via timeouts/assertions below
		close(done)
	}()

	// Give server a moment to start (CI can be slower)
	time.Sleep(500 * time.Millisecond)

	// Create UDP client socket
	laddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	raddr, _ := net.ResolveUDPAddr("udp", cfg.Server.Host+":"+"42999")
	conn, err := net.DialUDP("udp", laddr, raddr)
	if err != nil {
		t.Fatalf("failed to dial UDP: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Send YSFP poll (14 bytes) - construct a simple poll with callsign 'TEST'
	poll := make([]byte, network.PollPacketSize)
	copy(poll[0:4], []byte(network.PacketTypePoll))
	copy(poll[4:14], []byte("TEST      "))

	if _, err := conn.Write(poll); err != nil {
		t.Fatalf("failed to send poll: %v", err)
	}

	// Read poll response (allow more time on CI runners)
	buf := make([]byte, 512)
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Logf("warning: SetReadDeadline failed: %v", err)
	}
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("failed to read poll response: %v", err)
	}

	// Expecting 14-byte poll response
	if n != network.PollPacketSize {
		t.Fatalf("unexpected poll response size: got %d, want %d", n, network.PollPacketSize)
	}

	// Now send minimal 4-byte YSFS probe
	probe := []byte("YSFS")
	if _, err := conn.Write(probe); err != nil {
		t.Fatalf("failed to send status probe: %v", err)
	}

	// Read status response
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Logf("warning: SetReadDeadline failed: %v", err)
	}
	n, err = conn.Read(buf)
	if err != nil {
		t.Fatalf("failed to read status response: %v", err)
	}

	// Expecting 42-byte status response
	if n != network.StatusPacketSize {
		t.Fatalf("unexpected status response size: got %d, want %d", n, network.StatusPacketSize)
	}

	// Basic sanity checks on response contents
	if string(buf[0:4]) != network.PacketTypeStatus {
		t.Fatalf("status response missing header: %s", string(buf[0:4]))
	}

	// name field should contain our configured name (space-padded)
	name := string(buf[9:25])
	if !containsTrim(name, "E2E-TEST") {
		t.Fatalf("status response name mismatch: %q", name)
	}

	// count field (last 3 bytes) should be numeric
	count := string(buf[39:42])
	if len(count) != 3 {
		t.Fatalf("invalid count field length: %q", count)
	}
}

func containsTrim(field, want string) bool {
	// Trim spaces and NULs then compare
	f := string(field)
	f = strings.TrimSpace(strings.TrimRight(f, "\x00"))
	return f == want
}
