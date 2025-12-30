package statusMonitor

import (
	"testing"
)

func TestStatusMonitor_EchoDropTrigger(t *testing.T) {
	sm := NewStatusMonitor()
	sm.PlayerName = "TestPlayer"

	// Case 1: Not silenced
	sm.UpdatePlayerStatus("TestPlayer", []int{1}, 10) // 1 = Poison (not silence)
	actions := sm.CheckForActions()
	for _, action := range actions {
		if action.Type == "echo_drop" {
			t.Errorf("Unexpected echo_drop action when not silenced")
		}
	}

	// Case 2: Silenced with echo drops
	sm.UpdatePlayerStatus("TestPlayer", []int{6}, 10) // 6 = Silence
	actions = sm.CheckForActions()
	found := false
	for _, action := range actions {
		if action.Type == "echo_drop" {
			found = true
			if action.Priority != 10 {
				t.Errorf("Expected priority 10 for echo_drop, got %d", action.Priority)
			}
			if action.Target != "<me>" {
				t.Errorf("Expected target <me> for echo_drop, got %s", action.Target)
			}
		}
	}
	if !found {
		t.Errorf("Expected echo_drop action when silenced with echo drops")
	}

	// Case 3: Silenced without echo drops
	sm.UpdatePlayerStatus("TestPlayer", []int{6}, 0) // 6 = Silence, 0 echo drops
	actions = sm.CheckForActions()
	for _, action := range actions {
		if action.Type == "echo_drop" {
			t.Errorf("Unexpected echo_drop action when no echo drops available")
		}
	}
}
