package cureSelector

import (
	"PandaBot/internal/entity"
	"fmt"
	"math/rand"
	"testing"
)

// **Feature: automated-gameplay-assistant, Property 13: Optimal cure level calculation**
// Property 13: Optimal cure level calculation
// For any Party_Member needing healing, the Go_Server should calculate the optimal cure spell level based on missing HP, considering both MP efficiency for minor damage and maximum healing for critical damage
// Validates: Requirements 5.1, 5.2, 5.3

func TestOptimalCureLevelCalculation(t *testing.T) {
	// Run property-based test with 100 iterations
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_OptimalCureLevelCalculation", func(t *testing.T) {
			selector := NewCureSelector()

			// Generate random entity with varying HP levels
			entity := generateRandomEntity()
			availableMP := rand.Intn(500) + 100                  // 100-600 MP
			jobLevel := map[string]int{"WHM": rand.Intn(75) + 1} // Level 1-75

			// Test both efficiency and effectiveness priorities
			prioritizeEfficiency := rand.Float32() < 0.5

			option, err := selector.SelectOptimalCure(entity, nil, availableMP, jobLevel, prioritizeEfficiency)

			// Should successfully select a cure if entity needs healing and we have MP
			missingHP := calculateMissingHPFromEntity(entity)
			if missingHP > 0 && availableMP >= 8 { // Minimum MP for Cure
				if err != nil {
					t.Errorf("Should select cure for entity needing healing: %v", err)
				}

				if option == nil {
					t.Error("Should return cure option for entity needing healing")
				} else {
					// Verify the selection makes sense
					if option.MPCost > availableMP {
						t.Errorf("Selected cure costs %d MP but only have %d MP", option.MPCost, availableMP)
					}

					// For minor damage, should prefer efficient cures
					if missingHP < 100 && prioritizeEfficiency {
						if option.SpellName == "Cure VI" || option.SpellName == "Cure V" {
							t.Error("Should not use high-level cure for minor damage when prioritizing efficiency")
						}
					}

					// For critical damage, should prefer effective cures IF higher level cures are available
					if missingHP > 400 && !prioritizeEfficiency {
						if option.SpellName == "Cure" && availableMP > 88 {
							// Check if better cures are available at this level
							allOptions, _ := selector.GetAllCureOptions(availableMP, jobLevel)
							hasBetterOption := false
							for _, opt := range allOptions {
								if opt.SpellName != "Cure" && opt.HealAmount > option.HealAmount {
									hasBetterOption = true
									break
								}
							}
							if hasBetterOption {
								t.Error("Should use higher-level cure for critical damage when MP is available")
							}
						}
					}
				}
			}

			// Should return error if entity doesn't need healing
			if missingHP <= 0 {
				if err == nil {
					t.Error("Should return error for entity that doesn't need healing")
				}
			}
		})
	}
}

// **Feature: automated-gameplay-assistant, Property 14: Cure option optimization**
// Property 14: Cure option optimization
// For any healing scenario where multiple cure options are available, the Go_Server should consider both casting time and MP efficiency in selection
// Validates: Requirements 5.4

func TestCureOptionOptimization(t *testing.T) {
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_CureOptionOptimization", func(t *testing.T) {
			selector := NewCureSelector()

			// Set up scenario with multiple cure options available
			availableMP := 500                    // Plenty of MP for all options
			jobLevel := map[string]int{"WHM": 75} // Max level for all spells

			// Get all available options
			options, err := selector.GetAllCureOptions(availableMP, jobLevel)
			if err != nil {
				t.Fatalf("Failed to get cure options: %v", err)
			}

			if len(options) < 2 {
				t.Skip("Need at least 2 cure options for optimization test")
			}

			// Verify that options have different efficiency and time characteristics
			hasVariedEfficiency := false
			hasVariedTimeEfficiency := false

			for i := 0; i < len(options)-1; i++ {
				for j := i + 1; j < len(options); j++ {
					if options[i].Efficiency != options[j].Efficiency {
						hasVariedEfficiency = true
					}
					if options[i].TimeEfficiency != options[j].TimeEfficiency {
						hasVariedTimeEfficiency = true
					}
				}
			}

			if !hasVariedEfficiency {
				t.Error("Cure options should have varied MP efficiency")
			}

			if !hasVariedTimeEfficiency {
				t.Error("Cure options should have varied time efficiency")
			}

			// Test selection with different priorities
			entity := &entity.Entity{
				HPPercent: uint8(rand.Intn(80) + 1), // 1-80% HP
				HPMax:     1000,
			}
			entity.HPcurrent = uint32(float64(entity.HPMax) * float64(entity.HPPercent) / 100.0)

			efficiencyOption, _ := selector.SelectOptimalCure(entity, nil, availableMP, jobLevel, true)
			speedOption, _ := selector.SelectOptimalCure(entity, nil, availableMP, jobLevel, false)

			// When prioritizing efficiency vs speed, different cures might be selected
			if efficiencyOption != nil && speedOption != nil {
				// The efficiency-prioritized option should have better or equal MP efficiency
				if efficiencyOption.SpellName != speedOption.SpellName {
					if efficiencyOption.Efficiency < speedOption.Efficiency {
						t.Error("Efficiency-prioritized selection should have better MP efficiency")
					}
				}
			}
		})
	}
}

// **Feature: automated-gameplay-assistant, Property 15: MP validation before casting**
// Property 15: MP validation before casting
// For any cure spell selection made by the Go_Server, it should verify the caster has sufficient MP before sending the Action_Command
// Validates: Requirements 5.5

func TestMPValidationBeforeCasting(t *testing.T) {
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_MPValidationBeforeCasting", func(t *testing.T) {
			selector := NewCureSelector()

			// Generate random MP amount and spell
			availableMP := rand.Intn(200) // 0-200 MP
			spellNames := []string{"Cure", "Cure II", "Cure III", "Cure IV", "Cure V", "Cure VI"}
			spellName := spellNames[rand.Intn(len(spellNames))]

			// Validate MP
			canCast, err := selector.ValidateMP(spellName, availableMP)

			if err != nil {
				t.Errorf("MP validation should not error for valid spell: %v", err)
			}

			// Get spell info to check actual MP cost
			spellInfo, err := selector.GetCureSpellInfo(spellName)
			if err != nil {
				t.Fatalf("Should be able to get spell info: %v", err)
			}

			// Validation result should match actual MP availability
			expectedCanCast := availableMP >= int(spellInfo.MPCost)
			if canCast != expectedCanCast {
				t.Errorf("MP validation incorrect: available=%d, cost=%d, canCast=%v, expected=%v",
					availableMP, spellInfo.MPCost, canCast, expectedCanCast)
			}

			// When selecting optimal cure, should never return option that costs more than available MP
			entity := generateRandomEntity()
			jobLevel := map[string]int{"WHM": 75}

			option, err := selector.SelectOptimalCure(entity, nil, availableMP, jobLevel, true)
			if err == nil && option != nil {
				if option.MPCost > availableMP {
					t.Errorf("Selected cure costs %d MP but only have %d MP available", option.MPCost, availableMP)
				}
			}
		})
	}
}

func TestCureByDamageAmount(t *testing.T) {
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_CureByDamageAmount", func(t *testing.T) {
			selector := NewCureSelector()

			missingHP := rand.Intn(800) + 50    // 50-850 missing HP
			availableMP := rand.Intn(300) + 100 // 100-400 MP
			jobLevel := map[string]int{"WHM": rand.Intn(75) + 1}

			option, err := selector.SelectCureByDamage(missingHP, availableMP, jobLevel)

			// Should select appropriate cure if MP and level allow
			if availableMP >= 8 { // Minimum for Cure
				if err != nil {
					t.Errorf("Should select cure for damage amount %d: %v", missingHP, err)
				}

				if option != nil {
					// Should not massively overheal for small damage
					if missingHP < 100 && option.HealAmount > missingHP*3 {
						t.Errorf("Excessive overheal: %d damage, %d heal", missingHP, option.HealAmount)
					}

					// Should provide adequate healing for large damage IF higher level cures are available
					if missingHP > 400 && option.HealAmount < missingHP/2 {
						// Check if better cures are available at this level
						allOptions, _ := selector.GetAllCureOptions(availableMP, jobLevel)
						hasBetterOption := false
						for _, opt := range allOptions {
							if opt.HealAmount >= missingHP/2 {
								hasBetterOption = true
								break
							}
						}
						if hasBetterOption {
							t.Errorf("Inadequate healing: %d damage, %d heal (better options available)", missingHP, option.HealAmount)
						}
					}
				}
			}
		})
	}
}

// **Feature: automated-gameplay-assistant, Property 16: Curaga selection for multiple targets**
// Property 16: Curaga selection for multiple targets
// For any scenario where more than three Party_Members need healing simultaneously, the Go_Server should select appropriate curaga spells instead of individual cure spells for improved efficiency
// Validates: Requirements 5.6

func TestCuragaSelectionForMultipleTargets(t *testing.T) {
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_CuragaSelectionForMultipleTargets", func(t *testing.T) {
			selector := NewCureSelector()

			// Generate random party with varying numbers of members needing healing
			partySize := rand.Intn(6) + 2 // 2-7 party members
			partyMembers := make([]*entity.Entity, partySize)

			membersNeedingHealing := 0
			for j := 0; j < partySize; j++ {
				// Some members need healing, some don't
				needsHealing := rand.Float32() < 0.7 // 70% chance of needing healing

				var hpPercent uint8
				if needsHealing {
					hpPercent = uint8(rand.Intn(80) + 1) // 1-80% HP (needs healing)
					membersNeedingHealing++
				} else {
					hpPercent = uint8(rand.Intn(20) + 81) // 81-100% HP (doesn't need healing)
				}

				maxHP := uint32(rand.Intn(1000) + 500) // 500-1500 max HP
				currentHP := uint32(float64(maxHP) * float64(hpPercent) / 100.0)

				partyMembers[j] = &entity.Entity{
					Name:      fmt.Sprintf("Player%d", j),
					HPPercent: hpPercent,
					HPMax:     maxHP,
					HPcurrent: currentHP,
					MPPercent: uint8(rand.Intn(100) + 1),
					Job:       "WHM",
				}
			}

			availableMP := rand.Intn(400) + 100                  // 100-500 MP
			jobLevel := map[string]int{"WHM": rand.Intn(75) + 1} // Level 1-75

			// Test ShouldUseCuraga method
			shouldUseCuraga, curagaOption, err := selector.ShouldUseCuraga(partyMembers, availableMP, jobLevel)

			if membersNeedingHealing >= 3 {
				// Should recommend curaga when 3 or more members need healing
				if availableMP >= 60 && jobLevel["WHM"] >= 16 { // Minimum for Curaga
					if !shouldUseCuraga {
						t.Errorf("Should recommend curaga when %d members need healing", membersNeedingHealing)
					}

					if err != nil {
						t.Errorf("Should not error when curaga is available: %v", err)
					}

					if curagaOption != nil {
						// Verify it's actually a curaga spell
						if !isCuragaSpell(curagaOption.SpellName) {
							t.Errorf("Should select curaga spell, got: %s", curagaOption.SpellName)
						}

						// Verify MP cost is within available MP
						if curagaOption.MPCost > availableMP {
							t.Errorf("Selected curaga costs %d MP but only have %d MP", curagaOption.MPCost, availableMP)
						}
					}
				}
			} else if membersNeedingHealing < 3 {
				// Should not recommend curaga when fewer than 3 members need healing
				if shouldUseCuraga {
					t.Errorf("Should not recommend curaga when only %d members need healing", membersNeedingHealing)
				}
			}

			// Test direct curaga selection
			if membersNeedingHealing >= 3 {
				curagaOption, err := selector.SelectCuragaForMultipleTargets(partyMembers, availableMP, jobLevel)

				if availableMP >= 60 && jobLevel["WHM"] >= 16 { // Minimum for Curaga
					if err != nil {
						t.Errorf("Should select curaga for %d members needing healing: %v", membersNeedingHealing, err)
					}

					if curagaOption != nil {
						// Verify efficiency - curaga should be more MP efficient than individual cures
						totalIndividualCost := estimateIndividualCureCost(membersNeedingHealing)
						if curagaOption.MPCost > totalIndividualCost*2 {
							t.Errorf("Curaga should be more efficient: curaga cost %d vs estimated individual cost %d",
								curagaOption.MPCost, totalIndividualCost)
						}
					}
				}
			} else if membersNeedingHealing < 3 {
				// Should return error when not enough members need healing
				_, err := selector.SelectCuragaForMultipleTargets(partyMembers, availableMP, jobLevel)
				if err == nil {
					t.Error("Should return error when fewer than 3 members need healing")
				}
			}
		})
	}
}

func TestInvalidInputHandling(t *testing.T) {
	selector := NewCureSelector()

	// Test nil entity
	_, err := selector.SelectOptimalCure(nil, nil, 100, map[string]int{"WHM": 50}, true)
	if err == nil {
		t.Error("Should return error for nil entity")
	}

	// Test zero missing HP
	fullHealthEntity := &entity.Entity{
		HPPercent: 100,
		HPMax:     1000,
		HPcurrent: 1000,
	}
	_, err = selector.SelectOptimalCure(fullHealthEntity, nil, 100, map[string]int{"WHM": 50}, true)
	if err == nil {
		t.Error("Should return error for entity at full health")
	}

	// Test invalid spell name
	_, err = selector.ValidateMP("Invalid Spell", 100)
	if err == nil {
		t.Error("Should return error for invalid spell name")
	}

	// Test curaga with insufficient party members
	partyMembers := []*entity.Entity{
		{HPPercent: 50, HPMax: 1000, HPcurrent: 500},
		{HPPercent: 60, HPMax: 1000, HPcurrent: 600},
	}
	_, err = selector.SelectCuragaForMultipleTargets(partyMembers, 200, map[string]int{"WHM": 50})
	if err == nil {
		t.Error("Should return error for curaga with fewer than 3 party members")
	}
}

// Generator functions for property-based testing

func generateRandomEntity() *entity.Entity {
	hpPercent := uint8(rand.Intn(99) + 1)  // 1-99% HP (needs healing)
	maxHP := uint32(rand.Intn(1000) + 500) // 500-1500 max HP
	currentHP := uint32(float64(maxHP) * float64(hpPercent) / 100.0)

	return &entity.Entity{
		Name:      generateRandomName(),
		HPPercent: hpPercent,
		HPMax:     maxHP,
		HPcurrent: currentHP,
		MPPercent: uint8(rand.Intn(100) + 1),
		Job:       generateRandomJob(),
	}
}

func generateRandomName() string {
	names := []string{"Alice", "Bob", "Charlie", "Diana", "Eve", "Frank", "Grace", "Henry"}
	return names[rand.Intn(len(names))]
}

func generateRandomJob() string {
	jobs := []string{"WHM", "BLM", "WAR", "THF", "MNK", "RDM", "PLD", "DRK"}
	return jobs[rand.Intn(len(jobs))]
}

func calculateMissingHPFromEntity(entity *entity.Entity) int {
	if entity.HPMax > 0 {
		return int(entity.HPMax - entity.HPcurrent)
	}

	// Estimate based on percentage
	missingPercent := 100 - int(entity.HPPercent)
	estimatedMaxHP := 1000
	return (missingPercent * estimatedMaxHP) / 100
}

func isCuragaSpell(spellName string) bool {
	curagaSpells := []string{"Curaga", "Curaga II", "Curaga III", "Curaga IV", "Curaga V"}
	for _, curaga := range curagaSpells {
		if spellName == curaga {
			return true
		}
	}
	return false
}

func estimateIndividualCureCost(membersCount int) int {
	// Estimate cost of individual cure spells (average around 50 MP per cure)
	return membersCount * 50
}
