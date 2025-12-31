package statusMonitor

import (
	"testing"
)

func TestDesiredBuffMonitoring(t *testing.T) {
	sm := NewStatusMonitor()
	playerName := "Player1"

	// 1. Update party member to create them
	sm.UpdatePartyMember(playerName, 100, 100, 1, 123, []int{})

	// 2. Register a desired buff (Haste = 33)
	sm.RegisterDesiredBuff(playerName, 33, "haste")

	// 3. Check for actions - should include a manual_spell for haste
	actions := sm.CheckForActions()
	foundHaste := false
	for _, action := range actions {
		if action.Target == playerName && action.Spell == "haste" && action.Type == "manual_spell" {
			foundHaste = true
			break
		}
	}

	if !foundHaste {
		t.Errorf("Expected to find haste action for %s, but didn't", playerName)
	}

	// 4. Update player with the buff
	sm.UpdatePartyMember(playerName, 100, 100, 1, 123, []int{33})

	// 5. Check for actions - should NOT include haste now
	actions = sm.CheckForActions()
	foundHaste = false
	for _, action := range actions {
		if action.Target == playerName && action.Spell == "haste" {
			foundHaste = true
			break
		}
	}

	if foundHaste {
		t.Errorf("Did not expect to find haste action for %s after buff was applied", playerName)
	}

	// 6. Simulate buff expiring
	sm.UpdatePartyMember(playerName, 100, 100, 1, 123, []int{})

	// 7. Check for actions - should include haste again
	actions = sm.CheckForActions()
	foundHaste = false
	for _, action := range actions {
		if action.Target == playerName && action.Spell == "haste" {
			foundHaste = true
			break
		}
	}

	if !foundHaste {
		t.Errorf("Expected to find haste action for %s after buff expired, but didn't", playerName)
	}
}

func TestElementalBuffMonitoring(t *testing.T) {
	sm := NewStatusMonitor()
	playerName := "Player1"

	// 1. Update party member
	sm.UpdatePartyMember(playerName, 100, 100, 1, 123, []int{})

	// 2. Simulate "firebuffs" trigger registration (normally done by TriggerService)
	// firebuffs registers Protect (40), Shell (41), and Barfire (100)
	sm.RegisterDesiredBuff(playerName, 40, "protect")
	sm.RegisterDesiredBuff(playerName, 41, "shell")
	sm.RegisterDesiredBuff(playerName, 100, "firebuffs")

	// 3. Check for actions - should include all three
	actions := sm.CheckForActions()
	expectedBuffs := map[string]bool{"protect": false, "shell": false, "firebuffs": false}
	for _, action := range actions {
		if action.Target == playerName && action.Type == "manual_spell" {
			expectedBuffs[action.Spell] = true
		}
	}

	for buff, found := range expectedBuffs {
		if !found {
			t.Errorf("Expected to find %s action for %s, but didn't", buff, playerName)
		}
	}

	// 4. Update player with partial buffs
	sm.UpdatePartyMember(playerName, 100, 100, 1, 123, []int{40, 41}) // Only Protect and Shell

	// 5. Check for actions - should only include firebuffs
	actions = sm.CheckForActions()
	for _, action := range actions {
		if action.Spell == "protect" || action.Spell == "shell" {
			t.Errorf("Did not expect to find %s action after it was applied", action.Spell)
		}
	}

	foundFirebuffs := false
	for _, action := range actions {
		if action.Spell == "firebuffs" {
			foundFirebuffs = true
			break
		}
	}
	if !foundFirebuffs {
		t.Errorf("Expected to find firebuffs action, but didn't")
	}
}

func TestBuffLoopPrevention(t *testing.T) {
	sm := NewStatusMonitor()
	playerName := "Player1"

	// 1. Update party member
	sm.UpdatePartyMember(playerName, 100, 100, 3, 123, []int{40, 41}) // Has Protect/Shell, missing Barfire

	// 2. Register buffs as TriggerService does for firebuffs
	sm.RegisterDesiredBuff(playerName, 40, "protect")
	sm.RegisterDesiredBuff(playerName, 41, "shell")
	sm.RegisterDesiredBuff(playerName, 100, "barfire") // Now using individual trigger

	// 3. Check for actions
	actions := sm.CheckForActions()

	// Should only have barfire, not protect/shell or firebuffs
	foundBarfire := false
	for _, action := range actions {
		if action.Spell == "protect" || action.Spell == "shell" || action.Spell == "firebuffs" {
			t.Errorf("Did not expect to find %s action", action.Spell)
		}
		if action.Spell == "barfire" {
			foundBarfire = true
		}
	}

	if !foundBarfire {
		t.Errorf("Expected to find barfire action")
	}
}

func TestSelfBuffTargeting(t *testing.T) {
	sm := NewStatusMonitor()
	playerName := "Player1"
	botName := "BotCaster"

	// 1. Update party members (Player and Bot)
	sm.UpdatePartyMember(playerName, 100, 100, 1, 123, []int{})
	sm.UpdatePartyMember(botName, 100, 100, 3, 123, []int{}) // Bot is WHM (3)
	sm.UpdatePlayerStatus(botName, []int{}, 0)               // Set bot as the player running the addon

	// 2. Register a self-buff for the Player (e.g., Light Arts - 358)
	// Even if registered FOR Player1, it should target the caster
	// NOTE: In the real app, TriggerService now passes "<me>" for these.
	sm.RegisterDesiredBuff("<me>", 358, "light arts")

	// 3. Check for actions
	actions := sm.CheckForActions()

	foundLightArts := false
	for _, action := range actions {
		if action.Spell == "light arts" {
			foundLightArts = true
			if action.Target != botName {
				t.Errorf("Self-buff 'light arts' targeted %s, but should target the caster/bot %s", action.Target, botName)
			}
		}
	}

	if !foundLightArts {
		t.Errorf("Expected to find light arts action")
	}
}
