package brandmeister

import (
	"context"
	"fmt"
)

// FSM states
type State int

const (
	StateDisconnected State = iota
	StateAuthenticating
	StateConnected
)

// String returns a string representation of the state
func (s State) String() string {
	switch s {
	case StateDisconnected:
		return "Disconnected"
	case StateAuthenticating:
		return "Authenticating"
	case StateConnected:
		return "Connected"
	default:
		return "Unknown"
	}
}

// Sender interface for outgoing packets
type Sender interface {
	Send(data []byte) error
}

// FSM handles the BrandMeister YSF Direct handshake
type FSM struct {
	sender   Sender
	callsign string
	password string
	targetTG int
	dmrID    uint32

	State State
}

// NewFSM creates a new handshake state machine
func NewFSM(sender Sender, callsign, password string, targetTG int) *FSM {
	return &FSM{
		sender:   sender,
		callsign: callsign,
		password: password,
		targetTG: targetTG,
		State:    StateDisconnected,
	}
}

// Start initiates the handshake by sending YSFL
func (f *FSM) Start(ctx context.Context) error {
	packet := &YSFLPacket{Callsign: f.callsign}
	data, err := packet.Marshal()
	if err != nil {
		return err
	}

	f.State = StateAuthenticating
	return f.sender.Send(data)
}

// HandlePacket processes incoming packets and transitions state
func (f *FSM) HandlePacket(data []byte) error {
	if len(data) < 4 {
		return fmt.Errorf("packet too short")
	}

	switch {
	case len(data) >= 4 && string(data[:4]) == HeaderYSFK:
		packet := &YSFKPacket{}
		if err := packet.Unmarshal(data); err != nil {
			return err
		}

		hash := CalculateResponse(packet.Challenge, f.password)
		rptk := &RPTKPacket{
			RepeaterID: f.dmrID,
			Hash:       hash,
		}

		resp, err := rptk.Marshal()
		if err != nil {
			return err
		}
		return f.sender.Send(resp)

	case len(data) >= 6 && string(data[:6]) == HeaderYSFACK:
		// Send YSFO to link talkgroup
		options := fmt.Sprintf("group=%d", f.targetTG)
		ysfo := &YSFOPacket{Options: options}

		resp, err := ysfo.Marshal()
		if err != nil {
			return err
		}

		err = f.sender.Send(resp)
		if err == nil {
			f.State = StateConnected
		}
		return err

	default:
		// Ignore unknown packets during handshake
		return nil
	}
}

// SetDMRID sets the DMR ID for authentication
func (f *FSM) SetDMRID(id uint32) {
	f.dmrID = id
}
