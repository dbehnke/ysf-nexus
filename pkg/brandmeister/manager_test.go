package brandmeister

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestManagerHeartbeat(t *testing.T) {
	// Mock a local UDP server to receive heartbeats
	laddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to resolve: %v", err)
	}
	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer conn.Close()

	serverAddr := conn.LocalAddr().String()

	// Create manager with short heartbeat for testing
	cfg := Config{
		MasterServer:  serverAddr,
		Callsign:      "W1ABC",
		Password:      "pass",
		TargetTG:      3100,
		HeartbeatFreq: 100 * time.Millisecond,
	}

	mgr := NewManager(cfg)
	// Bind a local port for testing
	laddr_mgr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	serverUDPAddr := conn.LocalAddr().(*net.UDPAddr)
	mgr.conn, _ = net.DialUDP("udp", laddr_mgr, serverUDPAddr)

	mgr.fsm.State = StateConnected
	mgr.wg.Add(1) // for heartbeat
	go mgr.runHeartbeat()
	defer mgr.Stop()

	// Wait for YSFP
	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, addr, err := conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("Failed to read heartbeat: %v", err)
	}

	t.Logf("Received %d bytes from %v: %s", n, addr, string(buf[:n]))

	if string(buf[:4]) != HeaderYSFP {
		t.Errorf("Expected YSFP heartbeat, got %s", string(buf[:4]))
	}
}

func TestManagerStart(t *testing.T) {
	// Mock server
	laddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	conn, _ := net.ListenUDP("udp", laddr)
	defer conn.Close()

	serverAddr := conn.LocalAddr().String()

	cfg := Config{
		MasterServer: serverAddr,
		Callsign:     "W1ABC",
		Password:     "pass",
	}

	mgr := NewManager(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := mgr.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Check if YSFL was sent to mock server
	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err = conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("Failed to receive YSFL: %v", err)
	}

	if string(buf[:4]) != HeaderYSFL {
		t.Errorf("Expected YSFL, got %s", string(buf[:4]))
	}
}
