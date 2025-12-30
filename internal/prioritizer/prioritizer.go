package prioritizer

import (
	"PandaBot/internal/entity"
	"PandaBot/internal/protocol"
	"PandaBot/internal/spell"
	"PandaBot/internal/textParser"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ActionType represents the type of action to be performed
type ActionType int

const (
	ActionHeal ActionType = iota
	ActionStatusRemoval
	ActionBuff
	ActionDebuff
)

// PrioritizedAction represents an action with its calculated priority
type PrioritizedAction struct {
	Command    *protocol.ExecuteCommand
	ActionType ActionType
	Priority   int     // 1-10, higher is more urgent
	MPCost     int     // MP cost of the action
	Target     string  // Target player name
	Urgency    float64 // Calculated urgency score
	Timestamp  time.Time
}

// SpellPrioritizer manages and prioritizes spell casting actions
type SpellPrioritizer struct {
	actionQueue   []PrioritizedAction
	spellDatabase map[string]*spell.Spell
	partyState    map[string]*entity.Entity
	playerMP      int
	mu            sync.RWMutex
}

// NewSpellPrioritizer creates a new spell prioritizer
func NewSpellPrioritizer() *SpellPrioritizer {
	return &SpellPrioritizer{
		actionQueue:   make([]PrioritizedAction, 0),
		spellDatabase: make(map[string]*spell.Spell),
		partyState:    make(map[string]*entity.Entity),
		mu:            sync.RWMutex{},
	}
}

// UpdatePartyState updates the current party member states
func (sp *SpellPrioritizer) UpdatePartyState(statusUpdate *protocol.StatusUpdate) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	sp.playerMP = statusUpdate.PlayerMP

	// Update party member states
	for _, member := range statusUpdate.PartyMembers {
		entity := &entity.Entity{
			Name:      member.Name,
			HPPercent: uint8(member.HPPercent),
			MPPercent: uint8(member.MPPercent),
			Job:       member.Job,
			Distance:  member.Distance,
		}

		// Use actual HP/MP values if available (from Ashita v4)
		if member.HPActual > 0 {
			entity.HPcurrent = uint32(member.HPActual)
			// Estimate HPMax from percentage if we have both values
			if member.HPPercent > 0 {
				entity.HPMax = uint32(float64(member.HPActual) * 100.0 / float64(member.HPPercent))
			}
		}

		// Convert status effects
		if len(member.StatusEffects) > 0 {
			for i, effect := range member.StatusEffects {
				if i < len(entity.Buffs) {
					entity.Buffs[i] = uint16(effect)
				}
			}
		}

		sp.partyState[member.Name] = entity
	}
}

// AddTextParserActions is deprecated - use the centralized casting system instead
func (sp *SpellPrioritizer) AddTextParserActions(actions []textParser.TriggerEvent, sender string) error {
	// This method is deprecated. All trigger processing should now go through
	// the centralized casting system which handles prioritization internally.
	return fmt.Errorf("AddTextParserActions is deprecated - use centralized casting system instead")
}

// AddHealthBasedActions analyzes party health and adds cure actions as needed
func (sp *SpellPrioritizer) AddHealthBasedActions(cureThreshold int) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	for name, member := range sp.partyState {
		if member.NeedsCure(cureThreshold) {
			cureSpell := sp.selectOptimalCure(member)
			if cureSpell != "" {
				prioritizedAction := PrioritizedAction{
					Command: &protocol.ExecuteCommand{
						Command:  fmt.Sprintf("/ma \"%s\" %s", cureSpell, name),
						Target:   name,
						Priority: sp.calculateHealthPriority(member),
						Timeout:  5000,
						ID:       generateCommandID(),
					},
					ActionType: ActionHeal,
					Priority:   sp.calculateHealthPriority(member),
					Target:     name,
					Urgency:    sp.calculateHealthUrgency(member),
					Timestamp:  time.Now(),
				}

				if spellData, exists := sp.spellDatabase[cureSpell]; exists {
					prioritizedAction.MPCost = int(spellData.MPCost)
				}

				sp.actionQueue = append(sp.actionQueue, prioritizedAction)
			}
		}
	}

	sp.prioritizeQueue()
}

// AddStatusRemovalActions analyzes party status effects and adds removal actions
func (sp *SpellPrioritizer) AddStatusRemovalActions() {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	for name, member := range sp.partyState {
		naSpells := sp.determineRequiredNaSpells(member)
		for _, naSpell := range naSpells {
			prioritizedAction := PrioritizedAction{
				Command: &protocol.ExecuteCommand{
					Command:  fmt.Sprintf("/ma \"%s\" %s", naSpell, name),
					Target:   name,
					Priority: sp.calculateStatusPriority(naSpell),
					Timeout:  5000,
					ID:       generateCommandID(),
				},
				ActionType: ActionStatusRemoval,
				Priority:   sp.calculateStatusPriority(naSpell),
				Target:     name,
				Urgency:    sp.calculateStatusUrgency(naSpell),
				Timestamp:  time.Now(),
			}

			if spellData, exists := sp.spellDatabase[naSpell]; exists {
				prioritizedAction.MPCost = int(spellData.MPCost)
			}

			sp.actionQueue = append(sp.actionQueue, prioritizedAction)
		}
	}

	sp.prioritizeQueue()
}

// GetNextAction returns the highest priority action that can be executed
func (sp *SpellPrioritizer) GetNextAction() (*PrioritizedAction, error) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if len(sp.actionQueue) == 0 {
		return nil, fmt.Errorf("no actions in queue")
	}

	// Find the first action we can afford
	for i, action := range sp.actionQueue {
		if sp.canAffordAction(&action) {
			// Remove from queue and return
			sp.actionQueue = append(sp.actionQueue[:i], sp.actionQueue[i+1:]...)
			return &action, nil
		}
	}

	return nil, fmt.Errorf("insufficient MP for any queued actions")
}

// GetQueueStatus returns information about the current action queue
func (sp *SpellPrioritizer) GetQueueStatus() (int, int) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	totalActions := len(sp.actionQueue)
	affordableActions := 0

	for _, action := range sp.actionQueue {
		if sp.canAffordAction(&action) {
			affordableActions++
		}
	}

	return totalActions, affordableActions
}

// ClearQueue removes all actions from the queue
func (sp *SpellPrioritizer) ClearQueue() {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	sp.actionQueue = make([]PrioritizedAction, 0)
}

// prioritizeQueue sorts the action queue by priority and urgency
func (sp *SpellPrioritizer) prioritizeQueue() {
	sort.Slice(sp.actionQueue, func(i, j int) bool {
		a, b := sp.actionQueue[i], sp.actionQueue[j]

		// First sort by action type priority (healing > status removal > buffs)
		if a.ActionType != b.ActionType {
			return sp.getActionTypePriority(a.ActionType) > sp.getActionTypePriority(b.ActionType)
		}

		// Then by priority level
		if a.Priority != b.Priority {
			return a.Priority > b.Priority
		}

		// Then by urgency
		if a.Urgency != b.Urgency {
			return a.Urgency > b.Urgency
		}

		// Finally by timestamp (older first)
		return a.Timestamp.Before(b.Timestamp)
	})
}

// Helper functions

func (sp *SpellPrioritizer) determineActionType(spellName string) ActionType {
	switch {
	case isCureSpell(spellName):
		return ActionHeal
	case isNaSpell(spellName):
		return ActionStatusRemoval
	case isBuffSpell(spellName):
		return ActionBuff
	default:
		return ActionDebuff
	}
}

func (sp *SpellPrioritizer) getActionTypePriority(actionType ActionType) int {
	switch actionType {
	case ActionHeal:
		return 10
	case ActionStatusRemoval:
		return 8
	case ActionBuff:
		return 5
	case ActionDebuff:
		return 3
	default:
		return 1
	}
}

func (sp *SpellPrioritizer) selectOptimalCure(member *entity.Entity) string {
	missingHP := 100 - int(member.HPPercent)

	switch {
	case missingHP >= 80:
		return "Cure IV"
	case missingHP >= 60:
		return "Cure III"
	case missingHP >= 40:
		return "Cure II"
	case missingHP >= 20:
		return "Cure"
	default:
		return ""
	}
}

func (sp *SpellPrioritizer) calculateHealthPriority(member *entity.Entity) int {
	hp := int(member.HPPercent)
	switch {
	case hp <= 20:
		return 10 // Critical
	case hp <= 40:
		return 8 // High
	case hp <= 60:
		return 6 // Medium
	case hp <= 80:
		return 4 // Low
	default:
		return 2 // Very low
	}
}

func (sp *SpellPrioritizer) calculateHealthUrgency(member *entity.Entity) float64 {
	return float64(100-member.HPPercent) / 100.0
}

func (sp *SpellPrioritizer) determineRequiredNaSpells(member *entity.Entity) []string {
	var naSpells []string

	// This would need to check actual status effects from member.Buffs
	// For now, return empty slice as status effect detection needs more implementation

	return naSpells
}

func (sp *SpellPrioritizer) calculateStatusPriority(naSpell string) int {
	switch naSpell {
	case "Stona":
		return 9 // Stone is very dangerous
	case "Paralyna":
		return 8 // Paralysis prevents actions
	case "Silena":
		return 7 // Silence prevents magic
	case "Poisona":
		return 5 // Poison does damage over time
	case "Blindna":
		return 4 // Blind reduces accuracy
	default:
		return 3
	}
}

func (sp *SpellPrioritizer) calculateStatusUrgency(naSpell string) float64 {
	return float64(sp.calculateStatusPriority(naSpell)) / 10.0
}

func (sp *SpellPrioritizer) canAffordAction(action *PrioritizedAction) bool {
	return sp.playerMP >= action.MPCost
}

// Utility functions

func isCureSpell(spellName string) bool {
	cureSpells := []string{"Cure", "Cure II", "Cure III", "Cure IV", "Cure V", "Cure VI"}
	for _, cure := range cureSpells {
		if spellName == cure {
			return true
		}
	}
	return false
}

func isNaSpell(spellName string) bool {
	naSpells := []string{"Poisona", "Paralyna", "Blindna", "Silena", "Stona", "Viruna", "Cursna"}
	for _, na := range naSpells {
		if spellName == na {
			return true
		}
	}
	return false
}

func isBuffSpell(spellName string) bool {
	buffSpells := []string{"Protect", "Shell", "Haste", "Regen", "Barfira", "Barwatera", "Barthundra", "Barstonra", "Baraera", "Barblizzara"}
	for _, buff := range buffSpells {
		if spellName == buff {
			return true
		}
	}
	return false
}

func generateCommandID() string {
	return fmt.Sprintf("cmd_%d", time.Now().UnixNano())
}
