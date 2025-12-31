package server

import (
	"PandaBot/internal/casting"
	"PandaBot/internal/statusMonitor"
	"net"
	"testing"
)

func TestCentralizedMap_LegacyGC(t *testing.T) {
	sm := statusMonitor.NewStatusMonitor()
	server := &Server{
		statusMonitor: sm,
	}

	playerName := "TestPlayer"
	sm.UpdatePartyMember(playerName, 100, 100, 1, 123, []int{33}) // Has Haste (33)

	client := &Client{
		playerName: playerName,
		conn:       &mockMapConn{},
	}

	cmd := &QueuedCommand{
		ID:      "cmd1",
		Command: "/ma \"Haste\" " + playerName,
		Target:  playerName,
	}

	// Should be unnecessary because target has Haste
	if server.isCommandStillNecessary(client, cmd) {
		t.Errorf("Expected Haste command to be unnecessary for target with Haste status")
	}

	cmd2 := &QueuedCommand{
		ID:      "cmd2",
		Command: "/ma \"Protect V\" " + playerName,
		Target:  playerName,
	}

	// Should be necessary because target doesn't have Protect
	if !server.isCommandStillNecessary(client, cmd2) {
		t.Errorf("Expected Protect command to be necessary for target without Protect status")
	}
}

func TestCentralizedMap_CastingEngineGC(t *testing.T) {
	sm := statusMonitor.NewStatusMonitor()
	integration := casting.NewCastingServerIntegration()
	server := &Server{
		statusMonitor: sm,
		castingSystem: integration,
	}

	playerName := "TestPlayer"
	sm.UpdatePartyMember(playerName, 100, 100, 1, 123, []int{40}) // Has Protect (40)

	// Add a pending cast for Protect
	err := integration.GetCastingEngine().RequestCast(&casting.CastRequest{
		ID:        "cast1",
		Type:      casting.CastTypeManual,
		SpellName: "Protect V",
		Target:    playerName,
		Priority:  5,
		Context: &casting.CastContext{
			CasterName: "Caster",
			CasterMP:   1000,
		},
	})
	if err != nil {
		t.Fatalf("Failed to request cast: %v", err)
	}

	// Manually trigger GC
	server.validateCastingEngineCasts()

	// Check if cast was cancelled
	casts := integration.GetCastingEngine().GetActiveCasts()
	if cast, ok := casts["cast1"]; ok {
		if cast.State != casting.CastStateCancelled {
			t.Errorf("Expected Protect cast to be cancelled, but it is in state %v", cast.State)
		}
	} else {
		// If it's gone from the map, it might have been removed after completion/cancellation
		// But in validateCastingEngineCasts it just calls CancelCast
	}
}

type mockMapConn struct {
	net.Conn
}

func (m *mockMapConn) Close() error { return nil }
func (m *mockMapConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
}
func (m *mockMapConn) Write(b []byte) (n int, err error) { return len(b), nil }
