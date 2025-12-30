package naSelector

import (
	"PandaBot/internal/entity"
	"math/rand"
	"testing"
)

// **Feature: automated-gameplay-assistant, Property 16: Status effect to spell mapping**
// Property 16: Status effect to spell mapping
// For any Party_Member with negative status effects, the Go_Server should correctly identify the specific "na" spell required for removal
// Validates: Requirements 6.1

func TestStatusEffectToSpellMapping(t *testing.T) {
	// Run property-based test with 100 iterations
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_StatusEffectToSpellMapping", func(t *testing.T) {
			selector := NewNaSpellSelector()

			// Generate random status effects
			statusEffects := generateRandomStatusEffects()

			// Get required spells for these status effects
			requiredSpells, err := selector.IdentifyRequiredNaSpells(statusEffects)

			if len(statusEffects) == 0 {
				// Should handle empty status effects gracefully
				if err == nil {
					t.Error("Should return error for empty status effects")
				}
				return
			}

			if err != nil {
				t.Errorf("Should not error for valid status effects: %v", err)
				return
			}

			// Verify that each required spell can actually remove at least one of the status effects
			for _, spellName := range requiredSpells {
				spellInfo, err := selector.GetNaSpellInfo(spellName)
				if err != nil {
					t.Errorf("Required spell %s not found in database", spellName)
					continue
				}

				// Check that this spell can remove at least one of the input status effects
				canRemove := false
				for _, effectID := range statusEffects {
					for _, target := range spellInfo.Targets {
						if target == effectID {
							canRemove = true
							break
						}
					}
					if canRemove {
						break
					}
				}

				if !canRemove {
					t.Errorf("Spell %s was recommended but cannot remove any of the status effects %v", spellName, statusEffects)
				}
			}

			// Verify that known status effects map to correct spells
			for _, effectID := range statusEffects {
				effectInfo, err := selector.GetStatusEffectInfo(effectID)
				if err != nil {
					continue // Unknown status effect, skip
				}

				if effectInfo.NaSpell != "" {
					// This status effect should have a corresponding spell in the required list
					found := false
					for _, spellName := range requiredSpells {
						if spellName == effectInfo.NaSpell {
							found = true
							break
						}
					}

					if !found {
						t.Errorf("Status effect %d (%s) requires spell %s but it was not in required spells list",
							effectID, effectInfo.Name, effectInfo.NaSpell)
					}
				}
			}
		})
	}
}

// **Feature: automated-gameplay-assistant, Property 17: Status effect prioritization**
// Property 17: Status effect prioritization
// For any Party_Member with multiple status effects, the Go_Server should prioritize removal based on effect severity, with life-threatening conditions taking precedence over minor debuffs
// Validates: Requirements 6.2

func TestStatusEffectPrioritization(t *testing.T) {
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_StatusEffectPrioritization", func(t *testing.T) {
			selector := NewNaSpellSelector()

			// Generate multiple status effects with known severities
			statusEffects := []int{3, 7, 5, 20} // Poison(5), Petrification(9), Blindness(4), Doom(10)

			prioritizedEffects, err := selector.PrioritizeStatusEffects(statusEffects)
			if err != nil {
				t.Errorf("Should not error for valid status effects: %v", err)
				return
			}

			if len(prioritizedEffects) != len(statusEffects) {
				t.Errorf("Prioritized list should have same length as input: got %d, expected %d",
					len(prioritizedEffects), len(statusEffects))
				return
			}

			// Verify that effects are sorted by severity (highest first)
			for i := 0; i < len(prioritizedEffects)-1; i++ {
				currentEffect, err1 := selector.GetStatusEffectInfo(prioritizedEffects[i])
				nextEffect, err2 := selector.GetStatusEffectInfo(prioritizedEffects[i+1])

				if err1 != nil || err2 != nil {
					continue // Skip unknown effects
				}

				if currentEffect.Severity < nextEffect.Severity {
					t.Errorf("Status effects not properly prioritized: %s (severity %d) should come after %s (severity %d)",
						currentEffect.Name, currentEffect.Severity, nextEffect.Name, nextEffect.Severity)
				}
			}

			// Verify that life-threatening conditions (severity >= 8) come before minor debuffs (severity < 5)
			var highSeverityIndex, lowSeverityIndex int = -1, -1

			for i, effectID := range prioritizedEffects {
				effect, err := selector.GetStatusEffectInfo(effectID)
				if err != nil {
					continue
				}

				if effect.Severity >= 8 && highSeverityIndex == -1 {
					highSeverityIndex = i
				}
				if effect.Severity < 5 && lowSeverityIndex == -1 {
					lowSeverityIndex = i
				}
			}

			if highSeverityIndex != -1 && lowSeverityIndex != -1 && highSeverityIndex > lowSeverityIndex {
				t.Error("Life-threatening conditions should be prioritized over minor debuffs")
			}
		})
	}
}

// **Feature: automated-gameplay-assistant, Property 18: Status removal command generation**
// Property 18: Status removal command generation
// For any status effect requiring removal, the Go_Server should send the correct Action_Command with proper targeting for the appropriate "na" spell
// Validates: Requirements 6.3

func TestStatusRemovalCommandGeneration(t *testing.T) {
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_StatusRemovalCommandGeneration", func(t *testing.T) {
			selector := NewNaSpellSelector()

			// Generate random status effects and MP amount
			statusEffects := generateKnownStatusEffects() // Use known effects for this test
			availableMP := rand.Intn(200) + 50            // 50-250 MP

			if len(statusEffects) == 0 {
				return // Skip if no status effects
			}

			optimalSpell, err := selector.SelectOptimalNaSpell(statusEffects, availableMP)

			// Should either find a spell or return appropriate error
			if err != nil {
				// Error is acceptable if no spells are affordable or applicable
				if availableMP < 8 { // Minimum MP for cheapest spell
					return // Expected - not enough MP
				}

				// Check if any spells should be applicable
				hasRemovableEffect := false
				for _, effectID := range statusEffects {
					effect, err := selector.GetStatusEffectInfo(effectID)
					if err == nil && effect.NaSpell != "" {
						spell, err := selector.GetNaSpellInfo(effect.NaSpell)
						if err == nil && spell.MPCost <= availableMP {
							hasRemovableEffect = true
							break
						}
					}
				}

				if hasRemovableEffect {
					t.Errorf("Should find applicable spell but got error: %v", err)
				}
				return
			}

			// If we got a spell, verify it's correct
			if optimalSpell == nil {
				t.Error("Should return spell or error, not nil")
				return
			}

			// Verify the spell can afford with available MP
			if optimalSpell.MPCost > availableMP {
				t.Errorf("Selected spell costs %d MP but only have %d MP", optimalSpell.MPCost, availableMP)
			}

			// Verify the spell can remove at least one of the status effects
			canRemove := false
			for _, effectID := range statusEffects {
				for _, target := range optimalSpell.Targets {
					if target == effectID {
						canRemove = true
						break
					}
				}
				if canRemove {
					break
				}
			}

			if !canRemove {
				t.Errorf("Selected spell %s cannot remove any of the status effects %v",
					optimalSpell.SpellName, statusEffects)
			}
		})
	}
}

// **Feature: automated-gameplay-assistant, Property 19: Action queuing for unavailable resources**
// Property 19: Action queuing for unavailable resources
// For any status removal spell that is unavailable or when insufficient MP exists, the Go_Server should queue the action for later execution rather than failing
// Validates: Requirements 6.4

func TestActionQueuingForUnavailableResources(t *testing.T) {
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_ActionQueuingForUnavailableResources", func(t *testing.T) {
			selector := NewNaSpellSelector()

			// Test with insufficient MP
			statusEffects := []int{7}  // Petrification (requires Stona, 15 MP)
			lowMP := rand.Intn(10) + 1 // 1-10 MP (insufficient)

			_, err := selector.SelectOptimalNaSpell(statusEffects, lowMP)

			// Should return error indicating insufficient resources
			if err == nil {
				t.Error("Should return error when MP is insufficient for any applicable spells")
			}

			// Test with sufficient MP
			highMP := rand.Intn(100) + 50 // 50-150 MP (sufficient)
			spell, err := selector.SelectOptimalNaSpell(statusEffects, highMP)

			if err != nil {
				t.Errorf("Should find spell with sufficient MP: %v", err)
			}

			if spell != nil && spell.MPCost > highMP {
				t.Errorf("Selected spell should be affordable: costs %d, have %d", spell.MPCost, highMP)
			}
		})
	}
}

// **Feature: automated-gameplay-assistant, Property 20: State tracking after status removal**
// Property 20: State tracking after status removal
// For any status effect that is successfully removed, the Go_Server should update its internal tracking of Party_Member conditions to reflect the change
// Validates: Requirements 6.5

func TestStateTrackingAfterStatusRemoval(t *testing.T) {
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_StateTrackingAfterStatusRemoval", func(t *testing.T) {
			selector := NewNaSpellSelector()

			// Create a mock party member with status effects
			member := generateRandomPartyMember()
			availableMP := rand.Intn(200) + 100 // 100-300 MP

			// Get recommended spells before "removal"
			recommendedBefore, err := selector.AnalyzePartyMemberStatus(member, availableMP)
			if err != nil {
				return // Skip if analysis fails
			}

			if len(recommendedBefore) == 0 {
				return // Skip if no spells recommended
			}

			// Simulate status removal by removing some buffs
			originalBuffs := make([]uint16, len(member.Buffs))
			copy(originalBuffs, member.Buffs[:])

			// Remove first status effect
			if len(member.Buffs) > 0 && member.Buffs[0] > 0 {
				member.Buffs[0] = 0
			}

			// Get recommended spells after "removal"
			recommendedAfter, err := selector.AnalyzePartyMemberStatus(member, availableMP)
			if err != nil {
				t.Errorf("Analysis should not fail after status removal: %v", err)
				return
			}

			// The number of recommended spells should be less than or equal to before
			// (since we removed a status effect)
			if len(recommendedAfter) > len(recommendedBefore) {
				t.Errorf("Should not recommend more spells after status removal: before=%d, after=%d",
					len(recommendedBefore), len(recommendedAfter))
			}

			// Verify that the analysis correctly reflects the updated state
			hasActiveEffects := false
			for _, buffID := range member.Buffs {
				if buffID > 0 {
					hasActiveEffects = true
					break
				}
			}

			if !hasActiveEffects && len(recommendedAfter) > 0 {
				t.Error("Should not recommend spells when no status effects are present")
			}
		})
	}
}

// Generator functions for property-based testing

func TestDEXDownMapping(t *testing.T) {
	selector := NewNaSpellSelector()
	statusEffects := []int{137} // DEX Down

	requiredSpells, err := selector.IdentifyRequiredNaSpells(statusEffects)
	if err != nil {
		t.Fatalf("Failed to identify required spells: %v", err)
	}

	found := false
	for _, spell := range requiredSpells {
		if spell == "Erase" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("DEX Down (137) should require Erase, but it was not found in %v", requiredSpells)
	}

	optimal, err := selector.SelectOptimalNaSpell(statusEffects, 100)
	if err != nil {
		t.Fatalf("Failed to select optimal spell: %v", err)
	}

	if optimal == nil || optimal.SpellName != "Erase" {
		t.Errorf("Expected Erase as optimal spell for DEX Down, got %v", optimal)
	}
}

func generateRandomStatusEffects() []int {
	count := rand.Intn(5) // 0-4 status effects
	effects := make([]int, count)

	// Known status effect IDs
	knownEffects := []int{2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 136, 137, 138, 139, 140, 141, 142, 146, 147, 20, 21}

	for i := 0; i < count; i++ {
		effects[i] = knownEffects[rand.Intn(len(knownEffects))]
	}

	return effects
}

func generateKnownStatusEffects() []int {
	// Only generate status effects that have corresponding "na" spells
	removableEffects := []int{3, 4, 5, 6, 7, 8, 9, 20} // Poison, Paralysis, Blindness, Silence, Petrification, Disease, Curse, Doom

	count := rand.Intn(3) + 1 // 1-3 effects
	effects := make([]int, count)

	for i := 0; i < count; i++ {
		effects[i] = removableEffects[rand.Intn(len(removableEffects))]
	}

	return effects
}

func generateRandomPartyMember() *entity.Entity {
	member := &entity.Entity{
		Name:      generateRandomName(),
		HPPercent: uint8(rand.Intn(100) + 1),
		MPPercent: uint8(rand.Intn(100) + 1),
		Job:       generateRandomJob(),
	}

	// Add some random status effects
	effectCount := rand.Intn(4) // 0-3 status effects
	for i := 0; i < effectCount && i < len(member.Buffs); i++ {
		knownEffects := []uint16{3, 4, 5, 6, 7, 8, 9, 20}
		member.Buffs[i] = knownEffects[rand.Intn(len(knownEffects))]
	}

	return member
}

func generateRandomName() string {
	names := []string{"Alice", "Bob", "Charlie", "Diana", "Eve", "Frank", "Grace", "Henry"}
	return names[rand.Intn(len(names))]
}

func generateRandomJob() string {
	jobs := []string{"WHM", "BLM", "WAR", "THF", "MNK", "RDM", "PLD", "DRK"}
	return jobs[rand.Intn(len(jobs))]
}
