package repeater

import (
    "net"
    "time"
)

// IsMuted reports whether the repeater at the given address is currently muted.
// This helper is only compiled for tests.
func (m *Manager) IsMuted(addr *net.UDPAddr) bool {
    if v, ok := m.muted.Load(addr.String()); ok {
        if until, ok2 := v.(time.Time); ok2 {
            if until.IsZero() {
                return true
            }
            return time.Now().Before(until)
        }
        // unknown type stored, treat as muted
        return true
    }
    return false
}

// ClearActive clears the activeKey so tests can simulate the active repeater stopping.
// This helper is only compiled for tests.
func (m *Manager) ClearActive() {
    m.activeMu.Lock()
    m.activeKey = ""
    m.activeMu.Unlock()
}
