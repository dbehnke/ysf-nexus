package repeater

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"
)

// Repeater represents a connected YSF repeater
type Repeater struct {
	callsign     string
	address      *net.UDPAddr
	connected    time.Time
	lastSeen     time.Time
	talkStart    *time.Time
	packetCount  uint64
	bytesRx      uint64
	bytesTx      uint64
	isActive     bool
}

// NewRepeater creates a new repeater instance
func NewRepeater(callsign string, address *net.UDPAddr) *Repeater {
	now := time.Now()
	return &Repeater{
		callsign:  callsign,
		address:   address,
		connected: now,
		lastSeen:  now,
		isActive:  true,
	}
}

// Callsign returns the repeater's callsign
func (r *Repeater) Callsign() string {
	return r.callsign
}

// Address returns the repeater's network address
func (r *Repeater) Address() *net.UDPAddr {
	return r.address
}

// Connected returns when the repeater connected
func (r *Repeater) Connected() time.Time {
	return r.connected
}

// LastSeen returns when the repeater was last seen
func (r *Repeater) LastSeen() time.Time {
	return r.lastSeen
}

// PacketCount returns the total number of packets processed
func (r *Repeater) PacketCount() uint64 {
	return atomic.LoadUint64(&r.packetCount)
}

// BytesReceived returns total bytes received from this repeater
func (r *Repeater) BytesReceived() uint64 {
	return atomic.LoadUint64(&r.bytesRx)
}

// BytesTransmitted returns total bytes transmitted to this repeater
func (r *Repeater) BytesTransmitted() uint64 {
	return atomic.LoadUint64(&r.bytesTx)
}

// IsActive returns whether the repeater is currently active
func (r *Repeater) IsActive() bool {
	return r.isActive
}

// IsTalking returns whether the repeater is currently transmitting
func (r *Repeater) IsTalking() bool {
	return r.talkStart != nil
}

// TalkDuration returns how long the repeater has been talking
func (r *Repeater) TalkDuration() time.Duration {
	if r.talkStart == nil {
		return 0
	}
	return time.Since(*r.talkStart)
}

// UpdateLastSeen updates the last seen timestamp
func (r *Repeater) UpdateLastSeen() {
	r.lastSeen = time.Now()
}

// IncrementPacketCount increments the packet counter
func (r *Repeater) IncrementPacketCount() {
	atomic.AddUint64(&r.packetCount, 1)
}

// AddBytesReceived adds to the bytes received counter
func (r *Repeater) AddBytesReceived(bytes uint64) {
	atomic.AddUint64(&r.bytesRx, bytes)
}

// AddBytesTransmitted adds to the bytes transmitted counter
func (r *Repeater) AddBytesTransmitted(bytes uint64) {
	atomic.AddUint64(&r.bytesTx, bytes)
}

// StartTalking marks the repeater as starting to talk
func (r *Repeater) StartTalking() {
	now := time.Now()
	r.talkStart = &now
}

// StopTalking marks the repeater as stopping to talk and returns the talk duration
func (r *Repeater) StopTalking() time.Duration {
	if r.talkStart == nil {
		return 0
	}

	duration := time.Since(*r.talkStart)
	r.talkStart = nil
	return duration
}

// SetActive sets the active status of the repeater
func (r *Repeater) SetActive(active bool) {
	r.isActive = active
}

// IsTimedOut checks if the repeater has timed out
func (r *Repeater) IsTimedOut(timeout time.Duration) bool {
	return time.Since(r.lastSeen) > timeout
}

// Uptime returns how long the repeater has been connected
func (r *Repeater) Uptime() time.Duration {
	return time.Since(r.connected)
}

// Stats returns a snapshot of repeater statistics
func (r *Repeater) Stats() RepeaterStats {
	return RepeaterStats{
		Callsign:          r.callsign,
		Address:           r.address.String(),
		Connected:         r.connected,
		LastSeen:          r.lastSeen,
		PacketCount:       r.PacketCount(),
		BytesReceived:     r.BytesReceived(),
		BytesTransmitted:  r.BytesTransmitted(),
		IsActive:          r.isActive,
		IsTalking:         r.IsTalking(),
		TalkDuration:      r.TalkDuration(),
		Uptime:            r.Uptime(),
	}
}

// RepeaterStats represents repeater statistics
type RepeaterStats struct {
	Callsign          string        `json:"callsign"`
	Address           string        `json:"address"`
	Connected         time.Time     `json:"connected"`
	LastSeen          time.Time     `json:"last_seen"`
	PacketCount       uint64        `json:"packet_count"`
	BytesReceived     uint64        `json:"bytes_received"`
	BytesTransmitted  uint64        `json:"bytes_transmitted"`
	IsActive          bool          `json:"is_active"`
	IsTalking         bool          `json:"is_talking"`
	TalkDuration      time.Duration `json:"talk_duration"`
	Uptime            time.Duration `json:"uptime"`
}

// String returns a string representation of the repeater
func (r *Repeater) String() string {
	status := "idle"
	if r.IsTalking() {
		status = "talking"
	}
	if !r.isActive {
		status = "inactive"
	}

	return fmt.Sprintf("Repeater{Callsign: %s, Address: %s, Status: %s, Uptime: %v}",
		r.callsign, r.address.String(), status, r.Uptime())
}