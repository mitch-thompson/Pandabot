package buffSelector

import (
	"fmt"
	"strings"
	"sync"

	"PandaBot/internal/action"
	"PandaBot/internal/registry"
	"PandaBot/internal/spell"
)

// BuffOption represents a buff spell option with its effectiveness metrics
type BuffOption struct {
	SpellName   string
	Spell       *spell.Spell
	MPCost      int
	CastTime    float32
	Efficiency  float64 // MP efficiency (1/MPCost for simplicity)
	IsAreaSpell bool
}

// BuffSelector handles optimal buff spell selection based on job levels
type BuffSelector struct {
	spellDatabase map[string]*spell.Spell
	protectSpells []*spell.Spell
	shellSpells   []*spell.Spell
	barSpells     []*spell.Spell
	regenSpells   []*spell.Spell
	mu            sync.RWMutex
}

// NewBuffSelector creates a new buff selector with spell database
func NewBuffSelector() *BuffSelector {
	bs := &BuffSelector{
		spellDatabase: make(map[string]*spell.Spell),
		protectSpells: make([]*spell.Spell, 0),
		shellSpells:   make([]*spell.Spell, 0),
		barSpells:     make([]*spell.Spell, 0),
		regenSpells:   make([]*spell.Spell, 0),
	}

	// Initialize with default buff spells
	bs.initializeDefaultBuffSpells()

	return bs
}

// initializeDefaultBuffSpells sets up the default buff spell database
func (bs *BuffSelector) initializeDefaultBuffSpells() {
	bs.protectSpells = make([]*spell.Spell, 0)
	bs.shellSpells = make([]*spell.Spell, 0)
	bs.barSpells = make([]*spell.Spell, 0)
	bs.regenSpells = make([]*spell.Spell, 0)

	allSpells := registry.GetAllSpells()

	for _, buffSpell := range allSpells {
		if buffSpell.Type != spell.Enhancing {
			continue
		}

		bs.spellDatabase[buffSpell.English] = buffSpell

		// Categorize spells based on name patterns
		name := buffSpell.English
		if strings.HasPrefix(name, "Protect") {
			bs.protectSpells = append(bs.protectSpells, buffSpell)
		} else if strings.HasPrefix(name, "Shell") {
			bs.shellSpells = append(bs.shellSpells, buffSpell)
		} else if strings.HasPrefix(name, "Bar") {
			bs.barSpells = append(bs.barSpells, buffSpell)
		} else if strings.HasPrefix(name, "Regen") {
			bs.regenSpells = append(bs.regenSpells, buffSpell)
		}
	}
}

// SelectOptimalProtect selects the best Protect spell based on job levels and party size
func (bs *BuffSelector) SelectOptimalProtect(jobLevels map[string]int, availableMP int, partySize int) (*BuffOption, error) {
	return bs.selectOptimalProtectInternal(jobLevels, availableMP, partySize, false)
}

// selectOptimalProtectInternal is the internal implementation with fallback control
func (bs *BuffSelector) selectOptimalProtectInternal(jobLevels map[string]int, availableMP int, partySize int, fallbackAttempt bool) (*BuffOption, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	// Prefer area spells for party of 3 or more, OR if player is WHM (WHMs should always use area spells when available)
	// But if this is a fallback attempt, force single target preference
	preferArea := !fallbackAttempt && (partySize >= 3 || jobLevels["WHM"] > 0)
	var bestOption *BuffOption
	bestLevel := 0

	for _, spellObj := range bs.protectSpells {
		// Check MP requirement
		if int(spellObj.MPCost) > availableMP {
			continue
		}

		// Check job level requirement
		canCast := false
		for job, level := range jobLevels {
			if requiredLevel, exists := spellObj.LevelReq[job]; exists {
				if level >= requiredLevel {
					canCast = true
					break
				}
			}
		}

		if !canCast {
			continue
		}

		// Determine if this is an area spell (can only target self)
		isArea := spellObj.Targets&action.TargetSelf != 0

		// Skip if we prefer area but this isn't area, or vice versa
		if preferArea && !isArea {
			continue
		}
		if !preferArea && isArea {
			continue
		}

		// Select highest level spell available
		if spellObj.Priority > bestLevel {
			bestLevel = spellObj.Priority
			bestOption = &BuffOption{
				SpellName:   spellObj.English,
				Spell:       spellObj,
				MPCost:      int(spellObj.MPCost),
				CastTime:    spellObj.CastTime,
				Efficiency:  1.0 / float64(spellObj.MPCost), // Simple efficiency metric
				IsAreaSpell: isArea,
			}
		}
	}

	// If no area spell found but we prefer area, try single target (but only once)
	if bestOption == nil && preferArea && !fallbackAttempt {
		return bs.selectOptimalProtectInternal(jobLevels, availableMP, partySize, true)
	}

	if bestOption == nil {
		return nil, fmt.Errorf("no suitable Protect spell available")
	}

	return bestOption, nil
}

// SelectOptimalShell selects the best Shell spell based on job levels and party size
func (bs *BuffSelector) SelectOptimalShell(jobLevels map[string]int, availableMP int, partySize int) (*BuffOption, error) {
	return bs.selectOptimalShellInternal(jobLevels, availableMP, partySize, false)
}

// selectOptimalShellInternal is the internal implementation with fallback control
func (bs *BuffSelector) selectOptimalShellInternal(jobLevels map[string]int, availableMP int, partySize int, fallbackAttempt bool) (*BuffOption, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	// Prefer area spells for party of 3 or more, OR if player is WHM (WHMs should always use area spells when available)
	// But if this is a fallback attempt, force single target preference
	preferArea := !fallbackAttempt && (partySize >= 3 || jobLevels["WHM"] > 0)

	var bestOption *BuffOption
	bestLevel := 0

	for _, spellObj := range bs.shellSpells {
		// Check MP requirement
		if int(spellObj.MPCost) > availableMP {
			continue
		}

		// Check job level requirement
		canCast := false
		for job, level := range jobLevels {
			if requiredLevel, exists := spellObj.LevelReq[job]; exists {
				if level >= requiredLevel {
					canCast = true
					break
				}
			}
		}

		if !canCast {
			continue
		}

		// Determine if this is an area spell (can only target self)
		isArea := spellObj.Targets&action.TargetSelf != 0

		// Skip if we prefer area but this isn't area, or vice versa
		if preferArea && !isArea {
			continue
		}
		if !preferArea && isArea {
			continue
		}

		// Select highest level spell available
		if spellObj.Priority > bestLevel {
			bestLevel = spellObj.Priority
			bestOption = &BuffOption{
				SpellName:   spellObj.English,
				Spell:       spellObj,
				MPCost:      int(spellObj.MPCost),
				CastTime:    spellObj.CastTime,
				Efficiency:  1.0 / float64(spellObj.MPCost),
				IsAreaSpell: isArea,
			}
		}
	}

	// If no area spell found but we prefer area, try single target (but only once)
	if bestOption == nil && preferArea && !fallbackAttempt {
		return bs.selectOptimalShellInternal(jobLevels, availableMP, partySize, true)
	}

	if bestOption == nil {
		return nil, fmt.Errorf("no suitable Shell spell available")
	}

	return bestOption, nil
}

// SelectBarSpell selects the appropriate Bar spell for elemental resistance
func (bs *BuffSelector) SelectBarSpell(element string, jobLevels map[string]int, availableMP int) (*BuffOption, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	// Map element names to spell names
	elementMap := map[string]string{
		"fire":    "Barfira",
		"ice":     "Barblizzara",
		"wind":    "Baraera",
		"earth":   "Barstonra",
		"thunder": "Barthundra",
		"water":   "Barwatera",
	}

	spellName, exists := elementMap[element]
	if !exists {
		return nil, fmt.Errorf("unknown element: %s", element)
	}

	spellObj, exists := bs.spellDatabase[spellName]
	if !exists {
		return nil, fmt.Errorf("spell %s not found", spellName)
	}

	// Check MP requirement
	if int(spellObj.MPCost) > availableMP {
		return nil, fmt.Errorf("insufficient MP for %s", spellName)
	}

	// Check job level requirement
	canCast := false
	for job, level := range jobLevels {
		if requiredLevel, exists := spellObj.LevelReq[job]; exists {
			if level >= requiredLevel {
				canCast = true
				break
			}
		}
	}

	if !canCast {
		return nil, fmt.Errorf("insufficient job level for %s", spellName)
	}

	return &BuffOption{
		SpellName:   spellObj.English,
		Spell:       spellObj,
		MPCost:      int(spellObj.MPCost),
		CastTime:    spellObj.CastTime,
		Efficiency:  1.0 / float64(spellObj.MPCost),
		IsAreaSpell: false,
	}, nil
}

// GetOptimalBuffSequence returns the optimal sequence of buff spells for firebuffs, etc.
func (bs *BuffSelector) GetOptimalBuffSequence(buffType string, jobLevels map[string]int, availableMP int, partySize int) ([]*BuffOption, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	var sequence []*BuffOption

	// Get Protect spell
	protectOption, err := bs.SelectOptimalProtect(jobLevels, availableMP, partySize)
	if err == nil && protectOption != nil {
		sequence = append(sequence, protectOption)
		availableMP -= protectOption.MPCost
	}

	// Get Shell spell
	shellOption, err := bs.SelectOptimalShell(jobLevels, availableMP, partySize)
	if err == nil && shellOption != nil {
		sequence = append(sequence, shellOption)
		availableMP -= shellOption.MPCost
	}

	// Get elemental Bar spell based on buff type
	elementMap := map[string]string{
		"firebuffs":    "fire",
		"waterbuffs":   "water",
		"thunderbuffs": "thunder",
		"earthbuffs":   "earth",
		"windbuffs":    "wind",
		"icebuffs":     "ice",
	}

	if element, exists := elementMap[buffType]; exists {
		barOption, err := bs.SelectBarSpell(element, jobLevels, availableMP)
		if err == nil && barOption != nil {
			sequence = append(sequence, barOption)
			availableMP -= barOption.MPCost
		}
	}

	// Add Reraise if available and not already in sequence
	reraiseOption, err := bs.SelectOptimalReraise(jobLevels, availableMP)
	if err == nil && reraiseOption != nil {
		sequence = append(sequence, reraiseOption)
		availableMP -= reraiseOption.MPCost
	}

	// Add Auspice if WHM 50+
	if level, exists := jobLevels["WHM"]; exists && level >= 50 {
		auspiceSpell := bs.spellDatabase["Auspice"]
		if auspiceSpell != nil && int(auspiceSpell.MPCost) <= availableMP {
			sequence = append(sequence, &BuffOption{
				SpellName:  auspiceSpell.English,
				Spell:      auspiceSpell,
				MPCost:     int(auspiceSpell.MPCost),
				CastTime:   auspiceSpell.CastTime,
				Efficiency: 1.0 / float64(auspiceSpell.MPCost),
			})
			availableMP -= int(auspiceSpell.MPCost)
		}
	}

	if len(sequence) == 0 {
		return nil, fmt.Errorf("no buff spells available for %s", buffType)
	}

	return sequence, nil
}

// SelectOptimalReraise selects the highest Reraise spell available
func (bs *BuffSelector) SelectOptimalReraise(jobLevels map[string]int, availableMP int) (*BuffOption, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	var bestOption *BuffOption
	bestPriority := -1

	for _, spellObj := range bs.spellDatabase {
		if !strings.HasPrefix(spellObj.English, "Reraise") {
			continue
		}

		// Check MP requirement
		if int(spellObj.MPCost) > availableMP {
			continue
		}

		// Check job level requirement
		canCast := false
		for job, level := range jobLevels {
			if requiredLevel, exists := spellObj.LevelReq[job]; exists {
				if level >= requiredLevel {
					canCast = true
					break
				}
			}
		}

		if canCast && spellObj.Priority > bestPriority {
			bestPriority = spellObj.Priority
			bestOption = &BuffOption{
				SpellName:   spellObj.English,
				Spell:       spellObj,
				MPCost:      int(spellObj.MPCost),
				CastTime:    spellObj.CastTime,
				Efficiency:  1.0 / float64(spellObj.MPCost),
				IsAreaSpell: false,
			}
		}
	}

	if bestOption == nil {
		return nil, fmt.Errorf("no Reraise spell available")
	}

	return bestOption, nil
}

// SelectOptimalRegen selects the highest Regen spell available
func (bs *BuffSelector) SelectOptimalRegen(jobLevels map[string]int, availableMP int) (*BuffOption, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	var bestOption *BuffOption
	bestPriority := -1

	for _, spellObj := range bs.regenSpells {
		// Check MP requirement
		if int(spellObj.MPCost) > availableMP {
			continue
		}

		// Check job level requirement
		canCast := false
		for job, level := range jobLevels {
			if requiredLevel, exists := spellObj.LevelReq[job]; exists {
				if level >= requiredLevel {
					canCast = true
					break
				}
			}
		}

		if canCast && spellObj.Priority > bestPriority {
			bestPriority = spellObj.Priority
			bestOption = &BuffOption{
				SpellName:   spellObj.English,
				Spell:       spellObj,
				MPCost:      int(spellObj.MPCost),
				CastTime:    spellObj.CastTime,
				Efficiency:  1.0 / float64(spellObj.MPCost),
				IsAreaSpell: false,
			}
		}
	}

	if bestOption == nil {
		return nil, fmt.Errorf("no Regen spell available")
	}

	return bestOption, nil
}

// GetBuffSpellInfo returns information about a specific buff spell
func (bs *BuffSelector) GetBuffSpellInfo(spellName string) (*spell.Spell, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	spell, exists := bs.spellDatabase[spellName]
	if !exists {
		return nil, fmt.Errorf("buff spell %s not found in database", spellName)
	}

	return spell, nil
}
