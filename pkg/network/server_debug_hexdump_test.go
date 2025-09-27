package network

import (
	"bytes"
	"context"
	"log"
	"net"
	"testing"
	"time"
)

// Test that when debug=true the hexdump is printed for incoming YSF packets
func TestDebugHexdumpRx(t *testing.T) {
	// Setup server with debug=true and start
	s := NewServer("127.0.0.1", 43003)
	s.SetDebug(true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = s.Start(ctx)
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Capture logs
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(nil)

	// Dial and send a YSFD (data) like packet
	serverAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:43003")
	clientAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	c, err := net.DialUDP("udp", clientAddr, serverAddr)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer c.Close()

	payload := make([]byte, DataPacketSize)
	copy(payload[0:4], []byte(PacketTypeData))
	copy(payload[4:14], []byte("DBGCLIENT "))

	if _, err := c.Write(payload); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Allow server to process logs
	time.Sleep(150 * time.Millisecond)

	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("YSF RX")) {
		t.Fatalf("expected hexdump, got: %s", out)
	}
}
