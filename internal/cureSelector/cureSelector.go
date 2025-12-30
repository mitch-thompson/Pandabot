package cureSelector

import (
	"PandaBot/internal/entity"
	"PandaBot/internal/spell"
	"fmt"
	"log"
	"math"
	"sync"
)

// CureOption represents a cure spell option with its effectiveness metrics
type CureOption struct {
	SpellName      string
	Spell          *spell.Spell
	HealAmount     int
	MPCost         int
	CastTime       float32
	Efficiency     float64 // Heal per MP
	TimeEfficiency float64 // Heal per second
	OverhealWaste  float64 // Percentage of wasted healing
}

// CureSelector handles optimal cure spell selection
type CureSelector struct {
	spellDatabase map[string]*spell.Spell
	cureSpells    []*spell.Spell
	curagaSpells  []*spell.Spell
	mu            sync.RWMutex
}

// NewCureSelector creates a new cure selector with spell database
func NewCureSelector() *CureSelector {
	cs := &CureSelector{
		spellDatabase: make(map[string]*spell.Spell),
		cureSpells:    make([]*spell.Spell, 0),
		curagaSpells:  make([]*spell.Spell, 0),
	}

	// Initialize with default cure spells
	cs.initializeDefaultCureSpells()

	return cs
}

// initializeDefaultCureSpells sets up the default cure spell database
func (cs *CureSelector) initializeDefaultCureSpells() {
	cureSpells := []*spell.Spell{
		{
			English:    "Cure",
			ID:         1,
			MPCost:     8,
			CastTime:   2.0,
			Recast:     0,
			Type:       spell.Healing,
			Element:    spell.Light,
			Targets:    spell.TargetAlly,
			Priority:   1,
			HealAmount: 30,
			LevelReq:   map[string]int{"WHM": 1, "RDM": 3, "PLD": 5, "SCH": 5},
		},
		{
			English:    "Cure II",
			ID:         2,
			MPCost:     24,
			CastTime:   2.5,
			Recast:     0,
			Type:       spell.Healing,
			Element:    spell.Light,
			Targets:    spell.TargetAlly,
			Priority:   2,
			HealAmount: 160,
			LevelReq:   map[string]int{"WHM": 11, "RDM": 14, "PLD": 17, "SCH": 17},
		},
		{
			English:    "Cure III",
			ID:         3,
			MPCost:     46,
			CastTime:   3.0,
			Recast:     0,
			Type:       spell.Healing,
			Element:    spell.Light,
			Targets:    spell.TargetAlly,
			Priority:   3,
			HealAmount: 300,
			LevelReq:   map[string]int{"WHM": 21, "RDM": 26, "PLD": 30, "SCH": 30},
		},
		{
			English:    "Cure IV",
			ID:         4,
			MPCost:     88,
			CastTime:   3.5,
			Recast:     0,
			Type:       spell.Healing,
			Element:    spell.Light,
			Targets:    spell.TargetAlly,
			Priority:   4,
			HealAmount: 480,
			LevelReq:   map[string]int{"WHM": 41, "RDM": 48, "PLD": 55, "SCH": 55},
		},
		{
			English:    "Cure V",
			ID:         5,
			MPCost:     135,
			CastTime:   4.0,
			Recast:     0,
			Type:       spell.Healing,
			Element:    spell.Light,
			Targets:    spell.TargetAlly,
			Priority:   5,
			HealAmount: 680,
			LevelReq:   map[string]int{"WHM": 61},
		},
		{
			English:    "Cure VI",
			ID:         6,
			MPCost:     180,
			CastTime:   4.5,
			Recast:     0,
			Type:       spell.Healing,
			Element:    spell.Light,
			Targets:    spell.TargetAlly,
			Priority:   6,
			HealAmount: 900,
			LevelReq:   map[string]int{"WHM": 80},
		},
		// Curaga spells for AoE healing
		{
			English:    "Curaga",
			ID:         7,
			MPCost:     60,
			CastTime:   3.0,
			Recast:     0,
			Type:       spell.Healing,
			Element:    spell.Light,
			Targets:    spell.TargetSelf,
			Priority:   7,
			HealAmount: 90,
			LevelReq:   map[string]int{"WHM": 16},
		},
		{
			English:    "Curaga II",
			ID:         8,
			MPCost:     120,
			CastTime:   3.5,
			Recast:     0,
			Type:       spell.Healing,
			Element:    spell.Light,
			Targets:    spell.TargetSelf,
			Priority:   8,
			HealAmount: 300,
			LevelReq:   map[string]int{"WHM": 31},
		},
		{
			English:    "Curaga III",
			ID:         9,
			MPCost:     180,
			CastTime:   4.0,
			Recast:     0,
			Type:       spell.Healing,
			Element:    spell.Light,
			Targets:    spell.TargetSelf,
			Priority:   9,
			HealAmount: 450,
			LevelReq:   map[string]int{"WHM": 51},
		},
		{
			English:    "Curaga IV",
			ID:         10,
			MPCost:     260,
			CastTime:   4.5,
			Recast:     0,
			Type:       spell.Healing,
			Element:    spell.Light,
			Targets:    spell.TargetSelf,
			Priority:   10,
			HealAmount: 650,
			LevelReq:   map[string]int{"WHM": 71},
		},
		{
			English:    "Curaga V",
			ID:         11,
			MPCost:     320,
			CastTime:   5.0,
			Recast:     0,
			Type:       spell.Healing,
			Element:    spell.Light,
			Targets:    spell.TargetSelf,
			Priority:   11,
			HealAmount: 850,
			LevelReq:   map[string]int{"WHM": 91},
		},
	}

	for _, cureSpell := range cureSpells {
		cs.spellDatabase[cureSpell.English] = cureSpell

		// Separate cure and curaga spells based on spell name
		// Curaga spells have "ga" in their name and can only target self
		if cureSpell.Targets == spell.TargetSelf {
			cs.curagaSpells = append(cs.curagaSpells, cureSpell)
		} else {
			cs.cureSpells = append(cs.cureSpells, cureSpell)
		}
	}
}

// SelectOptimalCure determines the best cure spell for a given situation using urgency-weighted HP/MP efficiency
func (cs *CureSelector) SelectOptimalCure(target *entity.Entity, partyMembers []*entity.Entity, availableMP int, jobLevel map[string]int, prioritizeEfficiency bool) (*CureOption, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	log.Printf("[CURE SELECTOR] SelectOptimalCure called for target: %s",
		func() string {
			if target != nil {
				return target.Name
			} else {
				return "nil"
			}
		}())

	if target == nil {
		log.Printf("[CURE SELECTOR] Error: target entity is nil")
		return nil, fmt.Errorf("target entity cannot be nil")
	}

	missingHP := cs.calculateMissingHP(target)
	log.Printf("[CURE SELECTOR] Target %s missing HP: %d (HP: %d/%d, %d%%)",
		target.Name, missingHP, target.HPcurrent, target.HPMax, target.HPPercent)

	if missingHP <= 0 {
		log.Printf("[CURE SELECTOR] Target does not need healing (missing HP: %d)", missingHP)
		return nil, fmt.Errorf("target does not need healing")
	}

	maxHP := cs.estimateMaxHP(target)
	if maxHP <= 0 {
		maxHP = 1000 // Fallback
	}
	targetUrgency := float64(missingHP) / float64(maxHP)

	log.Printf("[CURE SELECTOR] Available MP: %d, Job levels: %v, Prioritize efficiency: %t",
		availableMP, jobLevel, prioritizeEfficiency)

	// Get available single-target cure options
	availableCureOptions := cs.getAvailableCureOptions(availableMP, jobLevel)
	log.Printf("[CURE SELECTOR] Found %d available single-target cure options", len(availableCureOptions))

	// Get available Curaga options
	availableCuragaOptions := cs.getAvailableCuragaOptions(availableMP, jobLevel)
	log.Printf("[CURE SELECTOR] Found %d available Curaga options", len(availableCuragaOptions))

	var bestOption *CureOption
	maxWeightedEfficiency := -1.0

	// Evaluate single-target cures
	for _, option := range availableCureOptions {
		actualHeal := option.HealAmount
		if actualHeal > missingHP {
			actualHeal = missingHP
		}
		effectiveHeal := float64(actualHeal) * targetUrgency
		weightedEfficiency := effectiveHeal / float64(option.MPCost)

		log.Printf("[CURE SELECTOR] Evaluating single %s: effective heal %.1f, weighted efficiency %.4f",
			option.SpellName, effectiveHeal, weightedEfficiency)

		if weightedEfficiency > maxWeightedEfficiency {
			maxWeightedEfficiency = weightedEfficiency
			bestOption = option
		}
	}

	// Evaluate Curaga options
	if len(partyMembers) > 0 {
		for _, option := range availableCuragaOptions {
			totalEffectiveHeal := 0.0
			for _, member := range partyMembers {
				mMissing := cs.calculateMissingHP(member)
				if mMissing <= 0 {
					continue
				}
				mMax := cs.estimateMaxHP(member)
				if mMax <= 0 {
					mMax = 1000
				}
				mUrgency := float64(mMissing) / float64(mMax)

				mActualHeal := option.HealAmount
				if mActualHeal > mMissing {
					mActualHeal = mMissing
				}
				totalEffectiveHeal += float64(mActualHeal) * mUrgency
			}

			weightedEfficiency := totalEffectiveHeal / float64(option.MPCost)
			log.Printf("[CURE SELECTOR] Evaluating Curaga %s: total effective heal %.1f, weighted efficiency %.4f",
				option.SpellName, totalEffectiveHeal, weightedEfficiency)

			if weightedEfficiency > maxWeightedEfficiency {
				maxWeightedEfficiency = weightedEfficiency
				bestOption = option
			}
		}
	}

	if bestOption != nil {
		log.Printf("[CURE SELECTOR] Selected best option: %s (weighted efficiency: %.4f)",
			bestOption.SpellName, maxWeightedEfficiency)
	} else {
		log.Printf("[CURE SELECTOR] No best option selected")
		return nil, fmt.Errorf("no suitable cure option found")
	}

	return bestOption, nil
}

// SelectCureByDamage selects cure spell based on missing HP amount
func (cs *CureSelector) SelectCureByDamage(missingHP int, availableMP int, jobLevel map[string]int) (*CureOption, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	log.Printf("[CURE SELECTOR] SelectCureByDamage called: missing HP=%d, available MP=%d, job levels=%v",
		missingHP, availableMP, jobLevel)

	if missingHP <= 0 {
		log.Printf("[CURE SELECTOR] No healing needed (missing HP: %d)", missingHP)
		return nil, fmt.Errorf("no healing needed")
	}

	availableOptions := cs.getAvailableCureOptions(availableMP, jobLevel)
	log.Printf("[CURE SELECTOR] Found %d available cure options for damage-based selection", len(availableOptions))

	if len(availableOptions) == 0 {
		log.Printf("[CURE SELECTOR] No cure spells available")
		return nil, fmt.Errorf("no cure spells available")
	}

	// Find the most appropriate cure spell for the damage amount
	var bestOption *CureOption
	bestScore := -1.0

	log.Printf("[CURE SELECTOR] Evaluating options for %d missing HP:", missingHP)

	for _, option := range availableOptions {
		// Calculate how well this cure matches the needed healing
		healRatio := float64(option.HealAmount) / float64(missingHP)

		// Prefer cures that heal close to the needed amount (avoid massive overheal)
		var score float64
		var scoreType string

		if healRatio >= 0.8 && healRatio <= 1.5 {
			// Good match - heals most of the damage without too much overheal
			score = 1.0 - math.Abs(healRatio-1.0)
			scoreType = "good_match"
		} else if healRatio > 1.5 {
			// Overheal penalty
			score = 0.5 / healRatio
			scoreType = "overheal_penalty"
		} else {
			// Underheal - apply severe penalty for inadequate healing
			// For large damage amounts (>300), heavily penalize insufficient healing
			if missingHP > 300 && healRatio < 0.5 {
				// Severe penalty for grossly inadequate healing on large damage
				score = healRatio * 0.1
				scoreType = "severe_underheal_penalty"
			} else {
				// Standard underheal penalty
				score = healRatio * 0.7
				scoreType = "standard_underheal_penalty"
			}
		}

		// Factor in MP efficiency
		finalScore := score * option.Efficiency

		log.Printf("[CURE SELECTOR]   %s: heal=%d, ratio=%.2f, base_score=%.3f (%s), efficiency=%.2f, final_score=%.3f",
			option.SpellName, option.HealAmount, healRatio, score, scoreType, option.Efficiency, finalScore)

		if finalScore > bestScore {
			bestScore = finalScore
			bestOption = option
			log.Printf("[CURE SELECTOR]   ^ New best option")
		}
	}

	if bestOption != nil {
		coverage := float64(bestOption.HealAmount) / float64(missingHP) * 100
		log.Printf("[CURE SELECTOR] Selected by damage: %s (heal: %d, cost: %d MP, coverage: %.1f%%, score: %.3f)",
			bestOption.SpellName, bestOption.HealAmount, bestOption.MPCost, coverage, bestScore)
	}

	return bestOption, nil
}

// ValidateMP checks if the caster has sufficient MP for a cure spell
func (cs *CureSelector) ValidateMP(spellName string, availableMP int) (bool, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	spell, exists := cs.spellDatabase[spellName]
	if !exists {
		return false, fmt.Errorf("spell %s not found in database", spellName)
	}

	return availableMP >= int(spell.MPCost), nil
}

// GetCureSpellInfo returns information about a specific cure spell
func (cs *CureSelector) GetCureSpellInfo(spellName string) (*spell.Spell, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	spell, exists := cs.spellDatabase[spellName]
	if !exists {
		return nil, fmt.Errorf("spell %s not found in database", spellName)
	}

	return spell, nil
}

// GetAllCureOptions returns all available cure options for given constraints
func (cs *CureSelector) GetAllCureOptions(availableMP int, jobLevel map[string]int) ([]*CureOption, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.getAvailableCureOptions(availableMP, jobLevel), nil
}

// SelectCuragaForMultipleTargets selects appropriate curaga spell when more than 3 party members need healing
func (cs *CureSelector) SelectCuragaForMultipleTargets(partyMembers []*entity.Entity, availableMP int, jobLevel map[string]int) (*CureOption, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if len(partyMembers) < 3 {
		return nil, fmt.Errorf("curaga selection requires 3 or more party members needing healing")
	}

	// Calculate average missing HP of party members needing healing
	totalMissingHP := 0
	membersNeedingHealing := 0

	for _, member := range partyMembers {
		missingHP := cs.calculateMissingHP(member)
		if missingHP > 0 {
			totalMissingHP += missingHP
			membersNeedingHealing++
		}
	}

	if membersNeedingHealing < 3 {
		return nil, fmt.Errorf("only %d party members need healing, curaga not efficient", membersNeedingHealing)
	}

	averageMissingHP := totalMissingHP / membersNeedingHealing

	// Get available curaga options
	availableCuragaOptions := cs.getAvailableCuragaOptions(availableMP, jobLevel)
	if len(availableCuragaOptions) == 0 {
		return nil, fmt.Errorf("no curaga spells available with current MP and job levels")
	}

	// Select the best curaga option based on average missing HP
	bestOption := cs.selectBestCuragaOption(availableCuragaOptions, averageMissingHP, membersNeedingHealing)

	return bestOption, nil
}

// ShouldUseCuraga determines if curaga is more efficient than individual cures
func (cs *CureSelector) ShouldUseCuraga(partyMembers []*entity.Entity, availableMP int, jobLevel map[string]int) (bool, *CureOption, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	// Count party members needing healing
	membersNeedingHealing := 0
	missingHPPerMember := make([]int, 0, len(partyMembers))
	for _, member := range partyMembers {
		missing := cs.calculateMissingHP(member)
		if missing > 0 {
			membersNeedingHealing++
			missingHPPerMember = append(missingHPPerMember, missing)
		}
	}

	// Consider curaga if 2 or more members need healing; prioritize by MP efficiency
	if membersNeedingHealing >= 2 {
		// Find best curaga option available
		availableCuragaOptions := cs.getAvailableCuragaOptions(availableMP, jobLevel)
		if len(availableCuragaOptions) == 0 {
			return false, nil, fmt.Errorf("no curaga spells available with current MP and job levels")
		}

		// Compute an efficiency-aware best curaga for this group
		// Use average missing HP for scoring like SelectCuragaForMultipleTargets
		totalMissing := 0
		for _, m := range missingHPPerMember {
			totalMissing += m
		}
		averageMissing := 0
		if membersNeedingHealing > 0 {
			averageMissing = totalMissing / membersNeedingHealing
		}

		bestCuraga := cs.selectBestCuragaOption(availableCuragaOptions, averageMissing, membersNeedingHealing)
		if bestCuraga == nil {
			return false, nil, fmt.Errorf("no suitable curaga option found")
		}

		// Estimate the total MP cost to individually cure each member once with an adequate cure
		// Adequate = cheapest cure whose HealAmount covers at least 60% of missing HP; if none, pick highest HealAmount available
		singleOptions := cs.getAvailableCureOptions(availableMP, jobLevel)
		if len(singleOptions) == 0 {
			// If we can't cast any single-target cures but can cast curaga, prefer curaga
			return true, bestCuraga, nil
		}

		estimateTotalSingles := 0
		for _, missing := range missingHPPerMember {
			// Choose cheapest adequate option
			var chosen *CureOption
			for _, opt := range singleOptions {
				// Track best candidate under simple rules
				if chosen == nil {
					chosen = opt
					continue
				}
				// Prefer options that meet adequacy threshold first
				curAdequate := float64(opt.HealAmount) >= float64(missing)*0.6
				chosenAdequate := float64(chosen.HealAmount) >= float64(missing)*0.6

				if curAdequate && !chosenAdequate {
					chosen = opt
				} else if curAdequate == chosenAdequate {
					// If both adequate or both not, pick lower MP cost; tie-break with higher heal
					if opt.MPCost < chosen.MPCost || (opt.MPCost == chosen.MPCost && opt.HealAmount > chosen.HealAmount) {
						chosen = opt
					}
				}
			}

			if chosen != nil {
				estimateTotalSingles += chosen.MPCost
			}
		}

		// Prioritize curaga only if it is cheaper or equal cost than doing singles
		if bestCuraga.MPCost <= estimateTotalSingles {
			return true, bestCuraga, nil
		}
	}

	return false, nil, nil
}

// Helper functions

func (cs *CureSelector) calculateMissingHP(target *entity.Entity) int {
	log.Printf("[CURE SELECTOR] calculateMissingHP: HPMax=%d, HPcurrent=%d, HPPercent=%d%%",
		target.HPMax, target.HPcurrent, target.HPPercent)

	// Prioritize absolute HP values if both HPMax and HPcurrent are properly set
	if target.HPMax > 0 && target.HPcurrent <= target.HPMax {
		missingHP := int(target.HPMax - target.HPcurrent)
		log.Printf("[CURE SELECTOR] Using actual HP values: %d - %d = %d missing HP",
			target.HPMax, target.HPcurrent, missingHP)
		return missingHP
	}

	// Fall back to percentage-based calculation
	if target.HPPercent < 100 {
		missingPercent := 100 - int(target.HPPercent)

		// Use a more realistic HP estimate based on job level if available
		estimatedMaxHP := cs.estimateMaxHP(target)
		missingHP := (missingPercent * estimatedMaxHP) / 100
		log.Printf("[CURE SELECTOR] Using percentage calculation: %d%% missing of %d estimated = %d missing HP",
			missingPercent, estimatedMaxHP, missingHP)
		return missingHP
	}

	// Check if we have actual HP values that indicate missing HP even if percentage says 100%
	if target.HPMax > 0 && target.HPcurrent < target.HPMax {
		missingHP := int(target.HPMax - target.HPcurrent)
		log.Printf("[CURE SELECTOR] HP percentage incorrect (100%% but actual values show missing HP): %d - %d = %d missing HP",
			target.HPMax, target.HPcurrent, missingHP)
		return missingHP
	}

	log.Printf("[CURE SELECTOR] No healing needed (HP: %d/%d, %d%%)",
		target.HPcurrent, target.HPMax, target.HPPercent)
	return 0
}

// estimateMaxHP provides a more accurate HP estimate based on job and level
func (cs *CureSelector) estimateMaxHP(target *entity.Entity) int {
	// Base HP estimates by job level (rough FFXI values)
	baseHP := 500 // Minimum reasonable HP

	if target.JobLevel > 0 {
		// Rough HP scaling: ~15-20 HP per level for most jobs
		levelBonus := int(target.JobLevel) * 18
		baseHP += levelBonus

		// Job-specific adjustments
		switch target.Job {
		case "WAR", "PLD", "DRK", "MNK":
			// Melee jobs tend to have higher HP
			baseHP = int(float64(baseHP) * 1.2)
		case "WHM", "BLM", "RDM", "SMN", "SCH":
			// Mage jobs tend to have lower HP
			baseHP = int(float64(baseHP) * 0.9)
		case "THF", "RNG", "BST", "BRD", "NIN", "COR", "DNC":
			// Support/ranged jobs have moderate HP
			baseHP = int(float64(baseHP) * 1.0)
		}
	}

	// Reasonable bounds (200-2000 HP range)
	if baseHP < 200 {
		baseHP = 200
	}
	if baseHP > 2000 {
		baseHP = 2000
	}

	return baseHP
}

func (cs *CureSelector) getAvailableCureOptions(availableMP int, jobLevel map[string]int) []*CureOption {
	var options []*CureOption

	for _, cureSpell := range cs.cureSpells {
		// Check MP requirement
		if int(cureSpell.MPCost) > availableMP {
			continue
		}

		// Check job level requirement
		canCast := false
		for job, level := range jobLevel {
			if requiredLevel, exists := cureSpell.LevelReq[job]; exists {
				if level >= requiredLevel {
					canCast = true
					break
				}
			}
		}

		if !canCast {
			continue
		}

		// Create cure option
		option := &CureOption{
			SpellName:      cureSpell.English,
			Spell:          cureSpell,
			HealAmount:     cureSpell.HealAmount,
			MPCost:         int(cureSpell.MPCost),
			CastTime:       cureSpell.CastTime,
			Efficiency:     float64(cureSpell.HealAmount) / float64(cureSpell.MPCost),
			TimeEfficiency: float64(cureSpell.HealAmount) / float64(cureSpell.CastTime),
		}

		options = append(options, option)
	}

	return options
}

func (cs *CureSelector) evaluateCureOptions(options []*CureOption, missingHP int, prioritizeEfficiency bool) []*CureOption {
	for _, option := range options {
		// Calculate overheal waste
		if option.HealAmount > missingHP {
			option.OverhealWaste = float64(option.HealAmount-missingHP) / float64(option.HealAmount)
		} else {
			option.OverhealWaste = 0
		}
	}

	return options
}

func (cs *CureSelector) selectBestOption(options []*CureOption, missingHP int, prioritizeEfficiency bool) *CureOption {
	if len(options) == 0 {
		log.Printf("[CURE SELECTOR] No options to select from")
		return nil
	}

	log.Printf("[CURE SELECTOR] Selecting best option from %d candidates (missing HP: %d, prioritize efficiency: %t)",
		len(options), missingHP, prioritizeEfficiency)

	var bestOption *CureOption
	bestScore := -1.0

	for _, option := range options {
		var score float64
		var scoreBreakdown string

		if prioritizeEfficiency {
			// For efficiency mode, balance MP efficiency with healing appropriateness
			healCoverage := math.Min(float64(option.HealAmount)/float64(missingHP), 1.0)

			// Prefer spells that heal 70-120% of missing HP (good coverage without excessive overheal)
			appropriatenessBonus := 1.0
			var bonusReason string

			if healCoverage >= 0.7 && healCoverage <= 1.2 {
				appropriatenessBonus = 1.5 // Bonus for appropriate healing amount
				bonusReason = "good_coverage"
			} else if healCoverage < 0.5 {
				appropriatenessBonus = 0.3 // Heavy penalty for insufficient healing
				bonusReason = "insufficient_healing"
			} else if healCoverage > 2.0 {
				appropriatenessBonus = 0.6 // Penalty for excessive overheal
				bonusReason = "excessive_overheal"
			} else {
				bonusReason = "standard"
			}

			// Balance efficiency with healing appropriateness
			score = option.Efficiency * appropriatenessBonus * (1.0 - option.OverhealWaste*0.3)
			scoreBreakdown = fmt.Sprintf("efficiency_mode: eff=%.2f * bonus=%.2f (%s) * waste_penalty=%.2f",
				option.Efficiency, appropriatenessBonus, bonusReason, (1.0 - option.OverhealWaste*0.3))
		} else {
			// Emergency mode: Prioritize healing effectiveness and speed, heavily penalize inadequate healing
			healCoverage := float64(option.HealAmount) / float64(missingHP)

			// For emergency healing, we want adequate coverage
			var healingScore float64
			var healingReason string

			if healCoverage >= 0.8 {
				// Good coverage - prefer this
				healingScore = 1.0 + (healCoverage-0.8)*0.5 // Bonus for good coverage
				healingReason = "good_coverage"
			} else if healCoverage >= 0.5 {
				// Moderate coverage - acceptable but not ideal
				healingScore = healCoverage * 1.2
				healingReason = "moderate_coverage"
			} else {
				// Poor coverage - heavily penalize for emergency situations
				healingScore = healCoverage * 0.2
				healingReason = "poor_coverage"
			}

			score = healingScore * option.TimeEfficiency * (1.0 - option.OverhealWaste*0.2)
			scoreBreakdown = fmt.Sprintf("emergency_mode: healing=%.2f (%s) * time_eff=%.2f * waste_penalty=%.2f",
				healingScore, healingReason, option.TimeEfficiency, (1.0 - option.OverhealWaste*0.2))
		}

		coverage := float64(option.HealAmount) / float64(missingHP) * 100
		log.Printf("[CURE SELECTOR]   %s: heal=%d (%.1f%% coverage), cost=%d MP, score=%.3f (%s)",
			option.SpellName, option.HealAmount, coverage, option.MPCost, score, scoreBreakdown)

		if score > bestScore {
			bestScore = score
			bestOption = option
			log.Printf("[CURE SELECTOR]   ^ New best option (score: %.3f)", score)
		}
	}

	if bestOption != nil {
		log.Printf("[CURE SELECTOR] Final selection: %s with score %.3f", bestOption.SpellName, bestScore)
	}

	return bestOption
}

func (cs *CureSelector) getAvailableCuragaOptions(availableMP int, jobLevel map[string]int) []*CureOption {
	var options []*CureOption

	for _, curagaSpell := range cs.curagaSpells {
		// Check MP requirement
		if int(curagaSpell.MPCost) > availableMP {
			continue
		}

		// Check job level requirement
		canCast := false
		for job, level := range jobLevel {
			if requiredLevel, exists := curagaSpell.LevelReq[job]; exists {
				if level >= requiredLevel {
					canCast = true
					break
				}
			}
		}

		if !canCast {
			continue
		}

		// Create curaga option
		option := &CureOption{
			SpellName:      curagaSpell.English,
			Spell:          curagaSpell,
			HealAmount:     curagaSpell.HealAmount,
			MPCost:         int(curagaSpell.MPCost),
			CastTime:       curagaSpell.CastTime,
			Efficiency:     float64(curagaSpell.HealAmount) / float64(curagaSpell.MPCost),
			TimeEfficiency: float64(curagaSpell.HealAmount) / float64(curagaSpell.CastTime),
		}

		options = append(options, option)
	}

	return options
}

func (cs *CureSelector) selectBestCuragaOption(options []*CureOption, averageMissingHP int, membersCount int) *CureOption {
	if len(options) == 0 {
		return nil
	}

	var bestOption *CureOption
	bestScore := -1.0

	for _, option := range options {
		// Calculate total healing efficiency for multiple targets
		totalHealingPotential := option.HealAmount * membersCount
		totalMPEfficiency := float64(totalHealingPotential) / float64(option.MPCost)

		// Calculate overheal waste based on average missing HP
		var overhealWaste float64
		if option.HealAmount > averageMissingHP {
			overhealWaste = float64(option.HealAmount-averageMissingHP) / float64(option.HealAmount)
		}

		// Score based on total efficiency and minimal overheal
		score := totalMPEfficiency * (1.0 - overhealWaste*0.4)

		// Bonus for healing more party members efficiently
		if membersCount > 4 {
			score *= 1.2
		}

		if score > bestScore {
			bestScore = score
			bestOption = option
		}
	}

	return bestOption
}

// AddCureSpell adds a new cure spell to the database
func (cs *CureSelector) AddCureSpell(cureSpell *spell.Spell) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.spellDatabase[cureSpell.English] = cureSpell

	// Add to appropriate slice based on target type
	if cureSpell.Targets&spell.TargetSelf != 0 {
		cs.curagaSpells = append(cs.curagaSpells, cureSpell)
	} else {
		cs.cureSpells = append(cs.cureSpells, cureSpell)
	}
}

// RemoveCureSpell removes a cure spell from the database
func (cs *CureSelector) RemoveCureSpell(spellName string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	delete(cs.spellDatabase, spellName)

	// Remove from cure spells slice
	for i, spell := range cs.cureSpells {
		if spell.English == spellName {
			cs.cureSpells = append(cs.cureSpells[:i], cs.cureSpells[i+1:]...)
			break
		}
	}

	// Remove from curaga spells slice
	for i, spell := range cs.curagaSpells {
		if spell.English == spellName {
			cs.curagaSpells = append(cs.curagaSpells[:i], cs.curagaSpells[i+1:]...)
			break
		}
	}
}
