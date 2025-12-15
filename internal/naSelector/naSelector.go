package naSelector

import (
	"PandaBot/internal/entity"
	"fmt"
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
	MPCost    int
	CastTime  float32
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
		{ID: 2, Name: "Sleep", Severity: 6, NaSpell: "Cure"},        // No direct "na" spell
		{ID: 3, Name: "Poison", Severity: 5, NaSpell: "Poisona"},
		{ID: 4, Name: "Paralysis", Severity: 8, NaSpell: "Paralyna"},
		{ID: 5, Name: "Blindness", Severity: 4, NaSpell: "Blindna"},
		{ID: 6, Name: "Silence", Severity: 7, NaSpell: "Silena"},
		{ID: 7, Name: "Petrification", Severity: 9, NaSpell: "Stona"},
		{ID: 8, Name: "Disease", Severity: 6, NaSpell: "Viruna"},
		{ID: 9, Name: "Curse", Severity: 7, NaSpell: "Cursna"},
		{ID: 10, Name: "Stun", Severity: 3, NaSpell: ""},       // No direct "na" spell
		{ID: 11, Name: "Bind", Severity: 4, NaSpell: "Erase"},       // No direct "na" spell
		{ID: 12, Name: "Weight", Severity: 3, NaSpell: "Erase"},     // No direct "na" spell
		{ID: 13, Name: "Slow", Severity: 4, NaSpell: "Erase"},       // Erase can remove
		{ID: 14, Name: "Attack Down", Severity: 3, NaSpell: "Erase"}, // Erase can remove
		{ID: 20, Name: "Doom", Severity: 10, NaSpell: "Cursna"},
	}
	
	for _, effect := range statusEffects {
		ns.statusEffectMap[effect.ID] = effect
	}
	
	// "Na" spell definitions
	naSpells := []*NaSpellOption{
		{
			SpellName: "Poisona",
			MPCost:    8,
			CastTime:  1.5,
			Targets:   []int{3}, // Poison
			Priority:  5,
		},
		{
			SpellName: "Paralyna",
			MPCost:    12,
			CastTime:  1.5,
			Targets:   []int{4}, // Paralysis
			Priority:  8,
		},
		{
			SpellName: "Blindna",
			MPCost:    8,
			CastTime:  1.5,
			Targets:   []int{5}, // Blindness
			Priority:  4,
		},
		{
			SpellName: "Silena",
			MPCost:    12,
			CastTime:  1.5,
			Targets:   []int{6}, // Silence
			Priority:  7,
		},
		{
			SpellName: "Stona",
			MPCost:    15,
			CastTime:  2.0,
			Targets:   []int{7}, // Petrification
			Priority:  9,
		},
		{
			SpellName: "Viruna",
			MPCost:    18,
			CastTime:  2.0,
			Targets:   []int{8}, // Disease
			Priority:  6,
		},
		{
			SpellName: "Cursna",
			MPCost:    20,
			CastTime:  2.5,
			Targets:   []int{9, 20}, // Curse, Doom
			Priority:  10,
		},
		{
			SpellName: "Erase",
			MPCost:    18,
			CastTime:  1.0,
			Targets:   []int{13, 14}, // Slow, Attack Down (and other enfeebling magic)
			Priority:  4,
		},
	}
	
	for _, spell := range naSpells {
		ns.naSpellMap[spell.SpellName] = spell
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
	for _, spell := range ns.naSpellMap {
		if spell.MPCost > availableMP {
			continue // Can't afford this spell
		}
		
		// Check if this spell can remove any of the status effects
		canRemove := false
		for _, effectID := range statusEffects {
			for _, target := range spell.Targets {
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
			applicableSpells = append(applicableSpells, spell)
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
	
	spell, exists := ns.naSpellMap[spellName]
	if !exists {
		return nil, fmt.Errorf("na spell %s not found", spellName)
	}
	
	return spell, nil
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
		if spell, exists := ns.naSpellMap[effect.NaSpell]; exists {
			if spell.MPCost <= availableMP && !processedSpells[effect.NaSpell] {
				recommendedSpells = append(recommendedSpells, effect.NaSpell)
				processedSpells[effect.NaSpell] = true
				availableMP -= spell.MPCost // Deduct MP for next calculations
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