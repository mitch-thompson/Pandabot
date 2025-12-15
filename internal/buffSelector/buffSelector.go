package buffSelector

import (
	"fmt"
	"log"
	"sync"

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
	mu            sync.RWMutex
}

// NewBuffSelector creates a new buff selector with spell database
func NewBuffSelector() *BuffSelector {
	bs := &BuffSelector{
		spellDatabase: make(map[string]*spell.Spell),
		protectSpells: make([]*spell.Spell, 0),
		shellSpells:   make([]*spell.Spell, 0),
		barSpells:     make([]*spell.Spell, 0),
	}

	// Initialize with default buff spells
	bs.initializeDefaultBuffSpells()

	return bs
}

// initializeDefaultBuffSpells sets up the default buff spell database
func (bs *BuffSelector) initializeDefaultBuffSpells() {
	// Protect spells (single target - can target party members)
	protectSpells := []*spell.Spell{
		{
			English:  "Protect",
			ID:       43,
			MPCost:   8,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Light,
			Targets:  spell.TargetPartyMember,
			Priority: 1,
			LevelReq: map[string]int{"WHM": 7, "RDM": 7, "PLD": 10},
		},
		{
			English:  "Protect II",
			ID:       44,
			MPCost:   18,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Light,
			Targets:  spell.TargetPartyMember,
			Priority: 2,
			LevelReq: map[string]int{"WHM": 27, "RDM": 27, "PLD": 30},
		},
		{
			English:  "Protect III",
			ID:       45,
			MPCost:   28,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Light,
			Targets:  spell.TargetPartyMember,
			Priority: 3,
			LevelReq: map[string]int{"WHM": 47, "RDM": 47, "PLD": 50},
		},
		{
			English:  "Protect IV",
			ID:       46,
			MPCost:   38,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Light,
			Targets:  spell.TargetPartyMember,
			Priority: 4,
			LevelReq: map[string]int{"WHM": 63, "RDM": 63, "PLD": 66},
		},
		{
			English:  "Protect V",
			ID:       47,
			MPCost:   48,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Light,
			Targets:  spell.TargetPartyMember,
			Priority: 5,
			LevelReq: map[string]int{"WHM": 75, "RDM": 75},
		},
	}

	// Protectra spells (area target - can only target self)
	protectraSpells := []*spell.Spell{
		{
			English:  "Protectra",
			ID:       125,
			MPCost:   9,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Light,
			Targets:  spell.TargetSelf,
			Priority: 1,
			LevelReq: map[string]int{"WHM": 11, "RDM": 11, "PLD": 15},
		},
		{
			English:  "Protectra II",
			ID:       126,
			MPCost:   20,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Light,
			Targets:  spell.TargetSelf,
			Priority: 2,
			LevelReq: map[string]int{"WHM": 31, "RDM": 31, "PLD": 35},
		},
		{
			English:  "Protectra III",
			ID:       127,
			MPCost:   32,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Light,
			Targets:  spell.TargetSelf,
			Priority: 3,
			LevelReq: map[string]int{"WHM": 47, "RDM": 47, "PLD": 55},
		},
		{
			English:  "Protectra IV",
			ID:       128,
			MPCost:   44,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Light,
			Targets:  spell.TargetSelf,
			Priority: 4,
			LevelReq: map[string]int{"WHM": 67, "RDM": 67},
		},
		{
			English:  "Protectra V",
			ID:       129,
			MPCost:   56,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Light,
			Targets:  spell.TargetSelf,
			Priority: 5,
			LevelReq: map[string]int{"WHM": 75, "RDM": 75},
		},
	}

	// Shell spells (single target - can target party members)
	shellSpells := []*spell.Spell{
		{
			English:  "Shell",
			ID:       48,
			MPCost:   8,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Light,
			Targets:  spell.TargetPartyMember,
			Priority: 1,
			LevelReq: map[string]int{"WHM": 17, "RDM": 17, "PLD": 20},
		},
		{
			English:  "Shell II",
			ID:       49,
			MPCost:   18,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Light,
			Targets:  spell.TargetPartyMember,
			Priority: 2,
			LevelReq: map[string]int{"WHM": 37, "RDM": 37, "PLD": 40},
		},
		{
			English:  "Shell III",
			ID:       50,
			MPCost:   28,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Light,
			Targets:  spell.TargetPartyMember,
			Priority: 3,
			LevelReq: map[string]int{"WHM": 57, "RDM": 57, "PLD": 60},
		},
		{
			English:  "Shell IV",
			ID:       51,
			MPCost:   38,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Light,
			Targets:  spell.TargetPartyMember,
			Priority: 4,
			LevelReq: map[string]int{"WHM": 68, "RDM": 68},
		},
		{
			English:  "Shell V",
			ID:       52,
			MPCost:   48,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Light,
			Targets:  spell.TargetPartyMember,
			Priority: 5,
			LevelReq: map[string]int{"WHM": 75, "RDM": 75},
		},
	}

	// Shellra spells (area target - can only target self)
	shellraSpells := []*spell.Spell{
		{
			English:  "Shellra",
			ID:       130,
			MPCost:   9,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Light,
			Targets:  spell.TargetSelf,
			Priority: 1,
			LevelReq: map[string]int{"WHM": 21, "RDM": 21, "PLD": 25},
		},
		{
			English:  "Shellra II",
			ID:       131,
			MPCost:   20,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Light,
			Targets:  spell.TargetSelf,
			Priority: 2,
			LevelReq: map[string]int{"WHM": 41, "RDM": 41, "PLD": 45},
		},
		{
			English:  "Shellra III",
			ID:       132,
			MPCost:   32,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Light,
			Targets:  spell.TargetSelf,
			Priority: 3,
			LevelReq: map[string]int{"WHM": 61, "RDM": 61},
		},
		{
			English:  "Shellra IV",
			ID:       133,
			MPCost:   44,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Light,
			Targets:  spell.TargetSelf,
			Priority: 4,
			LevelReq: map[string]int{"WHM": 68, "RDM": 68},
		},
		{
			English:  "Shellra V",
			ID:       134,
			MPCost:   56,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Light,
			Targets:  spell.TargetSelf,
			Priority: 5,
			LevelReq: map[string]int{"WHM": 75, "RDM": 75},
		},
	}

	// Bar spells (elemental resistance - self target only)
	barSpells := []*spell.Spell{
		{
			English:  "Barfira",
			ID:       53,
			MPCost:   8,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Ice,
			Targets:  spell.TargetSelf, // Bar spells can only target self
			Priority: 1,
			LevelReq: map[string]int{"WHM": 18, "RDM": 18},
		},
		{
			English:  "Barblizzara",
			ID:       54,
			MPCost:   8,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Fire,
			Targets:  spell.TargetSelf, // Bar spells can only target self
			Priority: 1,
			LevelReq: map[string]int{"WHM": 15, "RDM": 15},
		},
		{
			English:  "Baraera",
			ID:       55,
			MPCost:   8,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Earth,
			Targets:  spell.TargetSelf, // Bar spells can only target self
			Priority: 1,
			LevelReq: map[string]int{"WHM": 16, "RDM": 16},
		},
		{
			English:  "Barstonra",
			ID:       56,
			MPCost:   8,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Wind,
			Targets:  spell.TargetSelf, // Bar spells can only target self
			Priority: 1,
			LevelReq: map[string]int{"WHM": 19, "RDM": 19},
		},
		{
			English:  "Barthundra",
			ID:       57,
			MPCost:   8,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Water,
			Targets:  spell.TargetSelf, // Bar spells can only target self
			Priority: 1,
			LevelReq: map[string]int{"WHM": 20, "RDM": 20},
		},
		{
			English:  "Barwatera",
			ID:       58,
			MPCost:   8,
			CastTime: 3.0,
			Recast:   0,
			Type:     spell.Enhancing,
			Element:  spell.Thunder,
			Targets:  spell.TargetSelf, // Bar spells can only target self
			Priority: 1,
			LevelReq: map[string]int{"WHM": 17, "RDM": 17},
		},
	}

	// Add all spells to database and categorize them
	allSpells := append(protectSpells, protectraSpells...)
	allSpells = append(allSpells, shellSpells...)
	allSpells = append(allSpells, shellraSpells...)
	allSpells = append(allSpells, barSpells...)

	for _, buffSpell := range allSpells {
		bs.spellDatabase[buffSpell.English] = buffSpell

		// Categorize spells
		if buffSpell.English == "Protect" || buffSpell.English == "Protect II" ||
			buffSpell.English == "Protect III" || buffSpell.English == "Protect IV" ||
			buffSpell.English == "Protect V" || buffSpell.English == "Protectra" ||
			buffSpell.English == "Protectra II" || buffSpell.English == "Protectra III" ||
			buffSpell.English == "Protectra IV" || buffSpell.English == "Protectra V" {
			bs.protectSpells = append(bs.protectSpells, buffSpell)
		} else if buffSpell.English == "Shell" || buffSpell.English == "Shell II" ||
			buffSpell.English == "Shell III" || buffSpell.English == "Shell IV" ||
			buffSpell.English == "Shell V" || buffSpell.English == "Shellra" ||
			buffSpell.English == "Shellra II" || buffSpell.English == "Shellra III" ||
			buffSpell.English == "Shellra IV" || buffSpell.English == "Shellra V" {
			bs.shellSpells = append(bs.shellSpells, buffSpell)
		} else {
			bs.barSpells = append(bs.barSpells, buffSpell)
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

	log.Printf("[BUFF DEBUG] selectOptimalProtectInternal: jobLevels=%v, availableMP=%d, partySize=%d, preferArea=%t",
		jobLevels, availableMP, partySize, preferArea)

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
		isArea := spellObj.Targets&spell.TargetSelf != 0

		// Skip if we prefer area but this isn't area, or vice versa
		if preferArea && !isArea {
			continue
		}
		if !preferArea && isArea {
			continue
		}

		// Select highest level spell available
		if spellObj.Priority > bestLevel {
			log.Printf("[BUFF DEBUG] Selected better Protect: %s (Priority %d, MP: %d)", spellObj.English, spellObj.Priority, spellObj.MPCost)
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
		isArea := spellObj.Targets&spell.TargetSelf != 0

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

	log.Printf("[BUFF DEBUG] GetOptimalBuffSequence: buffType=%s, jobLevels=%v, availableMP=%d, partySize=%d",
		buffType, jobLevels, availableMP, partySize)

	var sequence []*BuffOption

	// Get Protect spell
	protectOption, err := bs.SelectOptimalProtect(jobLevels, availableMP, partySize)
	if err == nil && protectOption != nil {
		log.Printf("[BUFF DEBUG] Selected Protect: %s (MP: %d)", protectOption.SpellName, protectOption.MPCost)
		sequence = append(sequence, protectOption)
		availableMP -= protectOption.MPCost
	} else {
		log.Printf("[BUFF DEBUG] Failed to select Protect: %v", err)
	}

	// Get Shell spell
	shellOption, err := bs.SelectOptimalShell(jobLevels, availableMP, partySize)
	if err == nil && shellOption != nil {
		log.Printf("[BUFF DEBUG] Selected Shell: %s (MP: %d)", shellOption.SpellName, shellOption.MPCost)
		sequence = append(sequence, shellOption)
		availableMP -= shellOption.MPCost
	} else {
		log.Printf("[BUFF DEBUG] Failed to select Shell: %v", err)
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
			log.Printf("[BUFF DEBUG] Selected Bar spell: %s (MP: %d)", barOption.SpellName, barOption.MPCost)
			sequence = append(sequence, barOption)
		} else {
			log.Printf("[BUFF DEBUG] Failed to select Bar spell for %s: %v", element, err)
		}
	} else {
		log.Printf("[BUFF DEBUG] Unknown buff type: %s", buffType)
	}

	log.Printf("[BUFF DEBUG] Final sequence length: %d", len(sequence))
	for i, buff := range sequence {
		log.Printf("[BUFF DEBUG] Sequence[%d]: %s", i, buff.SpellName)
	}

	if len(sequence) == 0 {
		return nil, fmt.Errorf("no buff spells available for %s", buffType)
	}

	return sequence, nil
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
