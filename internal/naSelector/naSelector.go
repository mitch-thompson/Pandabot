package naSelector

import (
	"PandaBot/internal/entity"
	"PandaBot/internal/registry"
	"PandaBot/internal/spell"
	"fmt"
	"log"
	"sort"
	"sync"
)

// StatusEffect represents a negative status effect that needs removal
type StatusEffect struct {
	ID       int    // Status effect ID from game
	Name     string // Human readable name
	Severity int    // Priority level (1-10, higher is more urgent)
	NaSpell  string // Required "na" spell for removal
}

// NaSpellOption represents a "na" spell option with its effectiveness
type NaSpellOption struct {
	SpellName string
	Spell     *spell.Spell
	Targets   []int // Status effect IDs this spell can remove
	Priority  int   // Spell priority (higher is better)
}

// NaSpellSelector handles status effect removal spell selection
type NaSpellSelector struct {
	statusEffectMap map[int]*StatusEffect
	naSpellMap      map[string]*NaSpellOption
	mu              sync.RWMutex
}

// NewNaSpellSelector creates a new "na" spell selector with default mappings
func NewNaSpellSelector() *NaSpellSelector {
	ns := &NaSpellSelector{
		statusEffectMap: make(map[int]*StatusEffect),
		naSpellMap:      make(map[string]*NaSpellOption),
	}

	// Initialize default status effect mappings
	ns.initializeDefaultMappings()

	return ns
}

// initializeDefaultMappings sets up the default status effect to spell mappings
func (ns *NaSpellSelector) initializeDefaultMappings() {
	// Common negative status effects and their removal spells
	statusEffects := []*StatusEffect{
		{ID: 2, Name: "Sleep", Severity: 6, NaSpell: ""}, // Cure is not a "na" spell in this context
		{ID: 3, Name: "Poison", Severity: 5, NaSpell: "Poisona"},
		{ID: 4, Name: "Paralysis", Severity: 8, NaSpell: "Paralyna"},
		{ID: 5, Name: "Blindness", Severity: 4, NaSpell: "Blindna"},
		{ID: 6, Name: "Silence", Severity: 7, NaSpell: "Silena"},
		{ID: 7, Name: "Petrification", Severity: 9, NaSpell: "Stona"},
		{ID: 8, Name: "Disease", Severity: 6, NaSpell: "Viruna"},
		{ID: 31, Name: "Plague", Severity: 6, NaSpell: "Viruna"},
		{ID: 9, Name: "Curse", Severity: 7, NaSpell: "Cursna"},
		{ID: 10, Name: "Stun", Severity: 3, NaSpell: ""},             // No direct "na" spell
		{ID: 11, Name: "Bind", Severity: 4, NaSpell: "Erase"},        // No direct "na" spell
		{ID: 12, Name: "Weight", Severity: 3, NaSpell: "Erase"},      // No direct "na" spell
		{ID: 13, Name: "Slow", Severity: 4, NaSpell: "Erase"},        // Erase can remove
		{ID: 14, Name: "Attack Down", Severity: 3, NaSpell: "Erase"}, // Erase can remove
		{ID: 15, Name: "Defense Down", Severity: 3, NaSpell: "Erase"},
		{ID: 16, Name: "Magic Atk. Down", Severity: 3, NaSpell: "Erase"},
		{ID: 17, Name: "Magic Def. Down", Severity: 3, NaSpell: "Erase"},
		{ID: 136, Name: "STR Down", Severity: 3, NaSpell: "Erase"},
		{ID: 137, Name: "DEX Down", Severity: 3, NaSpell: "Erase"},
		{ID: 138, Name: "VIT Down", Severity: 3, NaSpell: "Erase"},
		{ID: 139, Name: "AGI Down", Severity: 3, NaSpell: "Erase"},
		{ID: 140, Name: "INT Down", Severity: 3, NaSpell: "Erase"},
		{ID: 141, Name: "MND Down", Severity: 3, NaSpell: "Erase"},
		{ID: 142, Name: "CHR Down", Severity: 3, NaSpell: "Erase"},
		{ID: 146, Name: "Accuracy Down", Severity: 3, NaSpell: "Erase"},
		{ID: 147, Name: "Evasion Down", Severity: 3, NaSpell: "Erase"},
		{ID: 20, Name: "Doom", Severity: 10, NaSpell: "Cursna"},
		{ID: 21, Name: "Amnesia", Severity: 7, NaSpell: ""}, // No direct "na" spell
	}

	for _, effect := range statusEffects {
		ns.statusEffectMap[effect.ID] = effect
	}

	// Names of "Na" spells and their target status effects
	spellTargets := map[string][]int{
		"Poisona":  {3},
		"Paralyna": {4},
		"Blindna":  {5},
		"Silena":   {6},
		"Stona":    {7},
		"Viruna":   {8, 31},
		"Cursna":   {9, 20},
		"Erase":    {11, 12, 13, 14, 15, 16, 17, 136, 137, 138, 139, 140, 141, 142, 146, 147},
	}

	for name, targets := range spellTargets {
		s, err := registry.GetSpell(name)
		if err != nil {
			log.Printf("[NA SELECTOR] Warning: spell %s not found in registry", name)
			continue
		}

		ns.naSpellMap[name] = &NaSpellOption{
			SpellName: name,
			Spell:     s,
			Targets:   targets,
			Priority:  s.Priority,
		}
	}
}

// IdentifyRequiredNaSpells determines which "na" spells are needed for given status effects
func (ns *NaSpellSelector) IdentifyRequiredNaSpells(statusEffects []int) ([]string, error) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	if len(statusEffects) == 0 {
		return nil, fmt.Errorf("no status effects provided")
	}

	var requiredSpells []string
	spellSet := make(map[string]bool)

	for _, effectID := range statusEffects {
		effect, exists := ns.statusEffectMap[effectID]
		if !exists {
			continue // Unknown status effect, skip
		}

		if effect.NaSpell != "" && !spellSet[effect.NaSpell] {
			requiredSpells = append(requiredSpells, effect.NaSpell)
			spellSet[effect.NaSpell] = true
		}
	}

	return requiredSpells, nil
}

// PrioritizeStatusEffects sorts status effects by severity (most urgent first)
func (ns *NaSpellSelector) PrioritizeStatusEffects(statusEffects []int) ([]int, error) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	if len(statusEffects) == 0 {
		return nil, fmt.Errorf("no status effects provided")
	}

	// Create a slice of effects with their severity for sorting
	type effectWithSeverity struct {
		ID       int
		Severity int
	}

	var effects []effectWithSeverity
	for _, effectID := range statusEffects {
		if effect, exists := ns.statusEffectMap[effectID]; exists {
			effects = append(effects, effectWithSeverity{
				ID:       effectID,
				Severity: effect.Severity,
			})
		}
	}

	// Sort by severity (highest first)
	sort.Slice(effects, func(i, j int) bool {
		return effects[i].Severity > effects[j].Severity
	})

	// Extract sorted IDs
	var sortedIDs []int
	for _, effect := range effects {
		sortedIDs = append(sortedIDs, effect.ID)
	}

	return sortedIDs, nil
}

// SelectOptimalNaSpell chooses the best "na" spell for given status effects and constraints
func (ns *NaSpellSelector) SelectOptimalNaSpell(statusEffects []int, availableMP int) (*NaSpellOption, error) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	if len(statusEffects) == 0 {
		return nil, fmt.Errorf("no status effects provided")
	}

	// Find all applicable spells
	var applicableSpells []*NaSpellOption
	for _, option := range ns.naSpellMap {
		if int(option.Spell.MPCost) > availableMP {
			continue // Can't afford this spell
		}

		// Check if this spell can remove any of the status effects
		canRemove := false
		for _, effectID := range statusEffects {
			for _, target := range option.Targets {
				if target == effectID {
					canRemove = true
					break
				}
			}
			if canRemove {
				break
			}
		}

		if canRemove {
			applicableSpells = append(applicableSpells, option)
		}
	}

	if len(applicableSpells) == 0 {
		return nil, fmt.Errorf("no applicable spells found for given status effects and MP")
	}

	// Select the highest priority spell
	bestSpell := applicableSpells[0]
	for _, spell := range applicableSpells[1:] {
		if spell.Priority > bestSpell.Priority {
			bestSpell = spell
		}
	}

	return bestSpell, nil
}

// GetStatusEffectInfo returns information about a specific status effect
func (ns *NaSpellSelector) GetStatusEffectInfo(effectID int) (*StatusEffect, error) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	effect, exists := ns.statusEffectMap[effectID]
	if !exists {
		return nil, fmt.Errorf("status effect %d not found", effectID)
	}

	return effect, nil
}

// GetNaSpellInfo returns information about a specific "na" spell
func (ns *NaSpellSelector) GetNaSpellInfo(spellName string) (*NaSpellOption, error) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	spellOption, exists := ns.naSpellMap[spellName]
	if !exists {
		return nil, fmt.Errorf("na spell %s not found", spellName)
	}

	return spellOption, nil
}

// AnalyzePartyMemberStatus analyzes a party member's status effects and recommends actions
func (ns *NaSpellSelector) AnalyzePartyMemberStatus(member *entity.Entity, availableMP int) ([]string, error) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	if member == nil {
		return nil, fmt.Errorf("party member cannot be nil")
	}

	// Extract status effect IDs from entity buffs
	var statusEffects []int
	for _, buffID := range member.Buffs {
		if buffID > 0 {
			statusEffects = append(statusEffects, int(buffID))
		}
	}

	if len(statusEffects) == 0 {
		return nil, nil // No status effects to remove
	}

	// Prioritize status effects by severity
	prioritizedEffects, err := ns.PrioritizeStatusEffects(statusEffects)
	if err != nil {
		return nil, err
	}

	// Get required spells for prioritized effects
	var recommendedSpells []string
	processedSpells := make(map[string]bool)

	for _, effectID := range prioritizedEffects {
		effect, exists := ns.statusEffectMap[effectID]
		if !exists || effect.NaSpell == "" {
			continue
		}

		// Check if we can afford this spell and haven't already recommended it
		if option, exists := ns.naSpellMap[effect.NaSpell]; exists {
			mpCost := int(option.Spell.MPCost)
			if mpCost <= availableMP && !processedSpells[effect.NaSpell] {
				recommendedSpells = append(recommendedSpells, effect.NaSpell)
				processedSpells[effect.NaSpell] = true
				availableMP -= mpCost // Deduct MP for next calculations
			}
		}
	}

	return recommendedSpells, nil
}

// AddStatusEffect adds or updates a status effect mapping
func (ns *NaSpellSelector) AddStatusEffect(effect *StatusEffect) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	ns.statusEffectMap[effect.ID] = effect
}

// AddNaSpell adds or updates a "na" spell mapping
func (ns *NaSpellSelector) AddNaSpell(spell *NaSpellOption) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	ns.naSpellMap[spell.SpellName] = spell
}
