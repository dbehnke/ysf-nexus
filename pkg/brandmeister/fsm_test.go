package brandmeister

import (
	"context"
	"encoding/binary"
	"testing"
	"time"
)

type mockManager struct {
	sentPackets [][]byte
}

func (m *mockManager) Send(data []byte) error {
	m.sentPackets = append(m.sentPackets, data)
	return nil
}

func TestHandshakeFSM(t *testing.T) {
	// The FSM should handle the sequence:
	// 1. Initial Start -> Send YSFL
	// 2. Received YSFK -> Send RPTK
	// 3. Received YSFACK -> Send YSFO (group link)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mgr := &mockManager{}
	fsm := NewFSM(mgr, "W1ABC", "password", 3100)

	// Step 1: Start
	err := fsm.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start FSM: %v", err)
	}

	if len(mgr.sentPackets) == 0 {
		t.Fatal("Expected YSFL to be sent on start")
	}

	if string(mgr.sentPackets[0][:4]) != HeaderYSFL {
		t.Errorf("Expected YSFL header, got %s", string(mgr.sentPackets[0][:4]))
	}

	// Step 2: Handle YSFK
	challenge := uint32(0x12345678)
	challengeData := make([]byte, 8)
	copy(challengeData[0:4], HeaderYSFK)
	binary.LittleEndian.PutUint32(challengeData[4:8], challenge)

	err = fsm.HandlePacket(challengeData)
	if err != nil {
		t.Fatalf("Failed to handle YSFK: %v", err)
	}

	if len(mgr.sentPackets) < 2 {
		t.Fatal("Expected RPTK to be sent after YSFK")
	}

	if string(mgr.sentPackets[1][:4]) != HeaderRPTK {
		t.Errorf("Expected RPTK header, got %s", string(mgr.sentPackets[1][:4]))
	}

	// Step 3: Handle YSFACK
	err = fsm.HandlePacket([]byte(HeaderYSFACK))
	if err != nil {
		t.Fatalf("Failed to handle YSFACK: %v", err)
	}

	if len(mgr.sentPackets) < 3 {
		t.Fatal("Expected YSFO to be sent after YSFACK")
	}

	if string(mgr.sentPackets[2][:4]) != HeaderYSFO {
		t.Errorf("Expected YSFO header, got %s", string(mgr.sentPackets[2][:4]))
	}

	if fsm.State != StateConnected {
		t.Errorf("Expected state to be Connected, got %v", fsm.State)
	}
}
