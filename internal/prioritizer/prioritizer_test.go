package prioritizer

import (
	"PandaBot/internal/protocol"
	"PandaBot/internal/textParser"
	"math/rand"
	"testing"
	"time"
)

// **Feature: automated-gameplay-assistant, Property 11: Action prioritization ranking**
// Property 11: Action prioritization ranking
// For any set of multiple concurrent healing, buffing, or status removal needs, the Spell_Prioritizer should rank actions by urgency (critical health > buffs) and importance (role-based priority)
// Validates: Requirements 4.1, 4.2, 4.3, 4.4

func TestActionPrioritizationRanking(t *testing.T) {
	// Run property-based test with 100 iterations
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_ActionPrioritizationRanking", func(t *testing.T) {
			prioritizer := NewSpellPrioritizer()
			
			// Generate random party state with various health levels
			statusUpdate := generateRandomStatusUpdate()
			prioritizer.UpdatePartyState(statusUpdate)
			
			// Add various types of actions
			prioritizer.AddHealthBasedActions(70) // Cure threshold at 70%
			prioritizer.AddStatusRemovalActions()
			
			// Add some buff actions via text parser
			buffActions := []textParser.TriggerAction{
				{
					Spells:   []string{"Protect", "Shell"},
					Priority: 3,
					Target:   "player1",
				},
			}
			prioritizer.AddTextParserActions(buffActions, "player1")
			
			// Get actions in priority order
			var retrievedActions []PrioritizedAction
			for {
				action, err := prioritizer.GetNextAction()
				if err != nil {
					break
				}
				retrievedActions = append(retrievedActions, *action)
			}
			
			// Verify prioritization rules
			if len(retrievedActions) > 1 {
				for i := 0; i < len(retrievedActions)-1; i++ {
					current := retrievedActions[i]
					next := retrievedActions[i+1]
					
					// Critical health should come before buffs
					if current.ActionType == ActionBuff && next.ActionType == ActionHeal {
						if next.Priority >= 8 { // Critical health priority
							t.Error("Critical healing should be prioritized over buffs")
						}
					}
					
					// Status removal should come before buffs
					if current.ActionType == ActionBuff && next.ActionType == ActionStatusRemoval {
						t.Error("Status removal should be prioritized over buffs")
					}
					
					// Higher priority should come first within same action type
					if current.ActionType == next.ActionType && current.Priority < next.Priority {
						t.Errorf("Higher priority action should come first: %d vs %d", current.Priority, next.Priority)
					}
				}
			}
		})
	}
}

// **Feature: automated-gameplay-assistant, Property 6: Multi-trigger prioritization**
// Property 6: Multi-trigger prioritization
// For any chat message containing multiple trigger words, the Spell_Prioritizer should determine an optimal casting sequence based on urgency and importance
// Validates: Requirements 2.4

func TestMultiTriggerPrioritization(t *testing.T) {
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_MultiTriggerPrioritization", func(t *testing.T) {
			prioritizer := NewSpellPrioritizer()
			
			// Set up party state
			statusUpdate := generateRandomStatusUpdate()
			prioritizer.UpdatePartyState(statusUpdate)
			
			// Create actions with multiple triggers (status removal + buffs)
			multiTriggerActions := []textParser.TriggerAction{
				{
					Spells:   []string{"Stona"}, // High priority status removal
					Priority: 8,
					Target:   "player1",
				},
				{
					Spells:   []string{"Protect", "Shell"}, // Lower priority buffs
					Priority: 3,
					Target:   "player1",
				},
			}
			
			prioritizer.AddTextParserActions(multiTriggerActions, "player1")
			
			// Get first action
			firstAction, err := prioritizer.GetNextAction()
			if err != nil {
				t.Fatalf("Expected to get first action, got error: %v", err)
			}
			
			// First action should be the higher priority one (Stona)
			if firstAction.ActionType != ActionStatusRemoval {
				t.Error("Status removal should be prioritized over buffs in multi-trigger scenario")
			}
			
			if firstAction.Priority < 8 {
				t.Errorf("Expected high priority action first, got priority %d", firstAction.Priority)
			}
		})
	}
}

// **Feature: automated-gameplay-assistant, Property 12: Resource optimization**
// Property 12: Resource optimization
// For any scenario where spell casting resources (MP) are limited, the Spell_Prioritizer should optimize MP usage and casting efficiency while maintaining healing effectiveness
// Validates: Requirements 4.5

func TestResourceOptimization(t *testing.T) {
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_ResourceOptimization", func(t *testing.T) {
			prioritizer := NewSpellPrioritizer()
			
			// Set up low MP scenario
			statusUpdate := &protocol.StatusUpdate{
				Timestamp:    time.Now().Unix(),
				PartyMembers: generateRandomPartyMembers(),
				PlayerMP:     rand.Intn(30) + 10, // Low MP (10-40)
				PlayerHP:     100,
				Zone:         "TestZone",
			}
			prioritizer.UpdatePartyState(statusUpdate)
			
			// Add various actions with different MP costs
			expensiveActions := []textParser.TriggerAction{
				{
					Spells:   []string{"Cure IV"}, // Expensive spell
					Priority: 7,
					Target:   "player1",
				},
			}
			
			cheapActions := []textParser.TriggerAction{
				{
					Spells:   []string{"Cure"}, // Cheap spell
					Priority: 6,
					Target:   "player2",
				},
			}
			
			prioritizer.AddTextParserActions(expensiveActions, "player1")
			prioritizer.AddTextParserActions(cheapActions, "player2")
			
			// Should be able to get at least one action (the affordable one)
			action, err := prioritizer.GetNextAction()
			if err != nil {
				// If no actions are affordable, that's also valid resource optimization
				totalActions, affordableActions := prioritizer.GetQueueStatus()
				if totalActions > 0 && affordableActions == 0 {
					// This is correct behavior - no actions are affordable
					return
				}
				t.Errorf("Expected to get an affordable action or have no affordable actions, got error: %v", err)
			}
			
			// If we got an action, it should be one we can afford
			if action != nil && action.MPCost > statusUpdate.PlayerMP {
				t.Errorf("Returned action costs %d MP but only have %d MP", action.MPCost, statusUpdate.PlayerMP)
			}
		})
	}
}

func TestHealthBasedPrioritization(t *testing.T) {
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_HealthBasedPrioritization", func(t *testing.T) {
			prioritizer := NewSpellPrioritizer()
			
			// Create party with different health levels
			members := []protocol.PartyMember{
				{
					Name:      "criticalPlayer",
					HPPercent: rand.Intn(20) + 1, // 1-20% HP (critical)
					MPPercent: 100,
					Job:       "WAR",
				},
				{
					Name:      "healthyPlayer",
					HPPercent: rand.Intn(20) + 80, // 80-100% HP (healthy)
					MPPercent: 100,
					Job:       "WHM",
				},
			}
			
			statusUpdate := &protocol.StatusUpdate{
				Timestamp:    time.Now().Unix(),
				PartyMembers: members,
				PlayerMP:     1000, // Plenty of MP
				PlayerHP:     100,
				Zone:         "TestZone",
			}
			
			prioritizer.UpdatePartyState(statusUpdate)
			prioritizer.AddHealthBasedActions(70)
			
			// Should prioritize critical health player
			action, err := prioritizer.GetNextAction()
			if err != nil {
				t.Fatalf("Expected to get healing action for critical player: %v", err)
			}
			
			if action.Target != "criticalPlayer" {
				t.Errorf("Expected critical player to be prioritized, got target: %s", action.Target)
			}
			
			if action.Priority < 8 {
				t.Errorf("Critical health should have high priority, got: %d", action.Priority)
			}
		})
	}
}

// Generator functions for property-based testing

func generateRandomStatusUpdate() *protocol.StatusUpdate {
	return &protocol.StatusUpdate{
		Timestamp:    time.Now().Unix(),
		PartyMembers: generateRandomPartyMembers(),
		PlayerMP:     rand.Intn(500) + 100, // 100-600 MP
		PlayerHP:     rand.Intn(50) + 50,   // 50-100% HP
		Zone:         "TestZone",
	}
}

func generateRandomPartyMembers() []protocol.PartyMember {
	count := rand.Intn(5) + 1 // 1-5 party members
	members := make([]protocol.PartyMember, count)
	
	jobs := []string{"WAR", "WHM", "BLM", "THF", "MNK", "RDM"}
	
	for i := 0; i < count; i++ {
		members[i] = protocol.PartyMember{
			Name:          generateRandomPlayerName(),
			HPPercent:     rand.Intn(100) + 1, // 1-100%
			MPPercent:     rand.Intn(100) + 1,
			StatusEffects: generateRandomStatusEffects(),
			Job:           jobs[rand.Intn(len(jobs))],
			Distance:      rand.Float32() * 50,
			LastUpdate:    time.Now(),
		}
	}
	
	return members
}

func generateRandomPlayerName() string {
	names := []string{"Alice", "Bob", "Charlie", "Diana", "Eve", "Frank"}
	return names[rand.Intn(len(names))] + string(rune('0'+rand.Intn(10)))
}

func generateRandomStatusEffects() []int {
	count := rand.Intn(4) // 0-3 status effects
	effects := make([]int, count)
	
	for i := 0; i < count; i++ {
		effects[i] = rand.Intn(100) + 1 // Random status effect IDs
	}
	
	return effects
}