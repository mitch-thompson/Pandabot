package autoActionService

import (
	"strings"
	"testing"
	"time"

	"PandaBot/internal/casting"
	"PandaBot/internal/statusMonitor"
)

func TestDecideNextAction_Silence(t *testing.T) {
	aas := NewAutoActionService(casting.NewCastingServerIntegration())
	sm := statusMonitor.NewStatusMonitor()
	playerName := "TestPlayer"

	// Ensure client exists
	csi := aas.castingSystem
	csi.RegisterClient(nil, playerName)

	// Mock player state
	sm.UpdatePartyMember(playerName, 100, 100, 1, 123, []int{6}) // Silenced
	sm.UpdatePlayerStatus(playerName, []int{6}, 5)               // Has 5 echo drops

	cmd, reason, err := aas.DecideNextAction(playerName, sm)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if cmd == nil || cmd.Command != "/item \"Echo Drops\" <me>" {
		t.Errorf("Expected echo drops command, got %v (reason: %s)", cmd, reason)
	}
}

func TestDecideNextAction_CriticalHeal(t *testing.T) {
	aas := NewAutoActionService(casting.NewCastingServerIntegration())
	sm := statusMonitor.NewStatusMonitor()
	playerName := "TestPlayer"

	// Ensure client exists
	csi := aas.castingSystem
	csi.RegisterClient(nil, playerName)

	// Mock party state - one member critical
	sm.UpdatePartyMember(playerName, 100, 100, 3, 123, []int{}) // WHM
	sm.UpdatePartyMemberWithMaxValues("Ally", 10, 100, 100, 1000, 1000, 1000, 1, 123, []int{})

	// Ensure casting engine has some MP
	csi.UpdateClientStatus(nil, 500, map[string]int{"WHM": 75})

	cmd, reason, err := aas.DecideNextAction(playerName, sm)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if cmd == nil || !strings.Contains(strings.ToLower(cmd.Command), "cure") {
		t.Errorf("Expected Cure command for critical ally, got %v (reason: %s)", cmd, reason)
	}
}

func TestDecideNextAction_MissingBuff(t *testing.T) {
	aas := NewAutoActionService(casting.NewCastingServerIntegration())
	sm := statusMonitor.NewStatusMonitor()
	playerName := "TestPlayer"

	// Ensure client exists
	csi := aas.castingSystem
	csi.RegisterClient(nil, playerName)

	sm.UpdatePartyMember(playerName, 100, 100, 3, 123, []int{})
	sm.RegisterDesiredBuff(playerName, 33, "haste", 50, time.Time{})

	// Ensure casting engine has some MP
	csi.UpdateClientStatus(nil, 500, map[string]int{"WHM": 75})

	cmd, reason, err := aas.DecideNextAction(playerName, sm)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if cmd == nil || !strings.Contains(strings.ToLower(cmd.Command), "haste") {
		t.Errorf("Expected Haste command, got %v (reason: %s)", cmd, reason)
	}
}
