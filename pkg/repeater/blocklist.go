package repeater

import (
	"strings"
	"sync"
)

// Blocklist manages blocked callsigns
type Blocklist struct {
	blocked map[string]bool
	mu      sync.RWMutex
}

// NewBlocklist creates a new blocklist
func NewBlocklist() *Blocklist {
	return &Blocklist{
		blocked: make(map[string]bool),
	}
}

// IsBlocked checks if a callsign is blocked
func (b *Blocklist) IsBlocked(callsign string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Normalize callsign for comparison
	normalized := strings.ToUpper(strings.TrimSpace(callsign))
	return b.blocked[normalized]
}

// Block adds a callsign to the blocklist
func (b *Blocklist) Block(callsign string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	normalized := strings.ToUpper(strings.TrimSpace(callsign))
	b.blocked[normalized] = true
}

// Unblock removes a callsign from the blocklist
func (b *Blocklist) Unblock(callsign string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	normalized := strings.ToUpper(strings.TrimSpace(callsign))
	delete(b.blocked, normalized)
}

// SetBlocked sets the entire blocklist
func (b *Blocklist) SetBlocked(callsigns []string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Clear existing
	b.blocked = make(map[string]bool)

	// Add new entries
	for _, callsign := range callsigns {
		normalized := strings.ToUpper(strings.TrimSpace(callsign))
		if normalized != "" {
			b.blocked[normalized] = true
		}
	}
}

// GetBlocked returns all blocked callsigns
func (b *Blocklist) GetBlocked() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var blocked []string
	for callsign := range b.blocked {
		blocked = append(blocked, callsign)
	}
	return blocked
}

// Count returns the number of blocked callsigns
func (b *Blocklist) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.blocked)
}

// Clear removes all entries from the blocklist
func (b *Blocklist) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.blocked = make(map[string]bool)
}