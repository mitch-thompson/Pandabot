package autoActionService

import (
	"strings"
	"testing"
	"time"

	"PandaBot/internal/casting"
	"PandaBot/internal/statusMonitor"
)

func TestDecideNextAction_TargetSelfBuffMonitoring(t *testing.T) {
	aas := NewAutoActionService(casting.NewCastingServerIntegration())
	sm := statusMonitor.NewStatusMonitor()
	playerName := "Caster"
	allyName := "Ally"

	// Ensure clients exist
	csi := aas.castingSystem
	csi.RegisterClient(nil, playerName)

	// Set up party members
	sm.UpdatePartyMember(playerName, 100, 100, 3, 123, []int{}) // Caster (WHM)
	sm.UpdatePartyMember(allyName, 100, 100, 1, 123, []int{40}) // Ally HAS Protect (ID 40)

	sm.UpdatePlayerStatus(playerName, []int{}, 0) // Caster DOES NOT have Protect
	csi.UpdateClientStatus(nil, 500, map[string]int{"WHM": 75})

	// Register "protect" for Ally.
	// In FFXI, if we use Protectra, it's TargetSelf, so it SHOULD be monitored on the Caster.
	sm.RegisterDesiredBuff(allyName, 40, "protect", 50, time.Time{})

	// Decide action
	cmd, reason, err := aas.DecideNextAction(playerName, sm)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Current behavior: It won't recast because Ally has Protect.
	// Expected behavior (per user): It should recast because Caster is missing Protect (since Protect resolves to Protectra V which is TargetSelf).

	if cmd == nil || !strings.Contains(strings.ToLower(cmd.Command), "protect") {
		t.Errorf("Expected Protectra recast because Caster is missing it, but got: %v (reason: %s)", cmd, reason)
	}
}
