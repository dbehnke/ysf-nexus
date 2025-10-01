package repeater

// ClearActive clears the activeKey so tests can simulate the active repeater stopping.
// This helper is only compiled for tests.
func (m *Manager) ClearActive() {
	m.activeMu.Lock()
	m.activeKey = ""
	m.activeMu.Unlock()
}
