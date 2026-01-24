package cureSelector

import (
	"PandaBot/internal/entity"
	"PandaBot/internal/registry"
	"PandaBot/internal/spell"
	"fmt"
	"math"
	"strings"
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
	mu sync.RWMutex
}

// NewCureSelector creates a new cure selector
func NewCureSelector() *CureSelector {
	return &CureSelector{}
}

// SelectOptimalCure determines the best cure spell for a given situation using urgency-weighted HP/MP efficiency
func (cs *CureSelector) SelectOptimalCure(target *entity.Entity, partyMembers []*entity.Entity, availableMP int, jobLevel map[string]int, prioritizeEfficiency bool) (*CureOption, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if target == nil {
		return nil, fmt.Errorf("target entity cannot be nil")
	}

	missingHP := cs.calculateMissingHP(target)

	if missingHP <= 0 {
		return nil, fmt.Errorf("target does not need healing")
	}

	maxHP := int(target.HPMax)
	if maxHP <= 0 {
		maxHP = 1000 // Fallback
	}
	targetUrgency := float64(missingHP) / float64(maxHP)

	// Get available single-target cure options
	availableCureOptions := cs.getAvailableCureOptions(availableMP, jobLevel)

	// Get available Curaga options
	availableCuragaOptions := cs.getAvailableCuragaOptions(availableMP, jobLevel)

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
				mMax := int(member.HPMax)
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

			if weightedEfficiency > maxWeightedEfficiency {
				maxWeightedEfficiency = weightedEfficiency
				bestOption = option
			}
		}
	}

	if bestOption != nil {
	} else {
		return nil, fmt.Errorf("no suitable cure option found")
	}

	return bestOption, nil
}

// SelectCureByDamage selects cure spell based on missing HP amount
func (cs *CureSelector) SelectCureByDamage(missingHP int, availableMP int, jobLevel map[string]int) (*CureOption, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if missingHP <= 0 {
		return nil, fmt.Errorf("no healing needed")
	}

	availableOptions := cs.getAvailableCureOptions(availableMP, jobLevel)

	if len(availableOptions) == 0 {
		return nil, fmt.Errorf("no cure spells available")
	}

	// Find the most appropriate cure spell for the damage amount
	var bestOption *CureOption
	bestScore := -1.0

	for _, option := range availableOptions {
		// Calculate how well this cure matches the needed healing
		healRatio := float64(option.HealAmount) / float64(missingHP)

		// Prefer cures that heal close to the needed amount (avoid massive overheal)
		var score float64
		var scoreType string
		_ = scoreType

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

		if finalScore > bestScore {
			bestScore = finalScore
			bestOption = option
		}
	}

	return bestOption, nil
}

// ValidateMP checks if the caster has sufficient MP for a cure spell
func (cs *CureSelector) ValidateMP(spellName string, availableMP int) (bool, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	s, err := registry.GetSpell(spellName)
	if err != nil {
		return false, err
	}

	return availableMP >= int(s.MPCost), nil
}

// GetCureSpellInfo returns information about a specific cure spell
func (cs *CureSelector) GetCureSpellInfo(spellName string) (*spell.Spell, error) {
	return registry.GetSpell(spellName)
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

	availableCuragaOptions := cs.getAvailableCuragaOptions(availableMP, jobLevel)
	if len(availableCuragaOptions) == 0 {
		return nil, fmt.Errorf("no curaga spells available with current MP and job levels")
	}
	bestOption := cs.selectBestCuragaOption(availableCuragaOptions, averageMissingHP, membersNeedingHealing)

	return bestOption, nil
}

// ShouldUseCuraga determines if curaga is more efficient than individual cures
func (cs *CureSelector) ShouldUseCuraga(partyMembers []*entity.Entity, availableMP int, jobLevel map[string]int) (bool, *CureOption, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	membersNeedingHealing := 0
	missingHPPerMember := make([]int, 0, len(partyMembers))
	for _, member := range partyMembers {
		missing := cs.calculateMissingHP(member)
		if missing > 0 {
			membersNeedingHealing++
			missingHPPerMember = append(missingHPPerMember, missing)
		}
	}

	if membersNeedingHealing >= 2 {
		availableCuragaOptions := cs.getAvailableCuragaOptions(availableMP, jobLevel)
		if len(availableCuragaOptions) == 0 {
			return false, nil, fmt.Errorf("no curaga spells available with current MP and job levels")
		}

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

		singleOptions := cs.getAvailableCureOptions(availableMP, jobLevel)
		if len(singleOptions) == 0 {
			// If we can't cast any single-target cures but can cast curaga, prefer curaga
			return true, bestCuraga, nil
		}

		estimateTotalSingles := 0
		for _, missing := range missingHPPerMember {
			var chosen *CureOption
			for _, opt := range singleOptions {
				if chosen == nil {
					chosen = opt
					continue
				}
				curAdequate := float64(opt.HealAmount) >= float64(missing)*0.6
				chosenAdequate := float64(chosen.HealAmount) >= float64(missing)*0.6

				if curAdequate && !chosenAdequate {
					chosen = opt
				} else if curAdequate == chosenAdequate {
					if opt.MPCost < chosen.MPCost || (opt.MPCost == chosen.MPCost && opt.HealAmount > chosen.HealAmount) {
						chosen = opt
					}
				}
			}

			if chosen != nil {
				estimateTotalSingles += chosen.MPCost
			}
		}
		if bestCuraga.MPCost <= estimateTotalSingles {
			return true, bestCuraga, nil
		}
	}

	return false, nil, nil
}

func (cs *CureSelector) calculateMissingHP(target *entity.Entity) int {
	if target.HPMax > 0 {
		missingHP := int(target.HPMax) - int(target.HPcurrent)
		if missingHP < 0 {
			missingHP = 0
		}
		return missingHP
	}

	return 0
}

func (cs *CureSelector) getAvailableCureOptions(availableMP int, jobLevel map[string]int) []*CureOption {
	var options []*CureOption

	allSpells := registry.GetAllSpells()
	for _, s := range allSpells {
		if s.Type != spell.Healing {
			continue
		}

		if !strings.HasPrefix(s.English, "Cure") {
			continue
		}

		if int(s.MPCost) > availableMP {
			continue
		}

		canCast := false
		for job, level := range jobLevel {
			if requiredLevel, exists := s.LevelReq[job]; exists {
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
			SpellName:      s.English,
			Spell:          s,
			HealAmount:     s.HealAmount,
			MPCost:         int(s.MPCost),
			CastTime:       s.CastTime,
			Efficiency:     float64(s.HealAmount) / float64(s.MPCost),
			TimeEfficiency: float64(s.HealAmount) / float64(s.CastTime),
		}

		options = append(options, option)
	}

	return options
}

func (cs *CureSelector) getAvailableCuragaOptions(availableMP int, jobLevel map[string]int) []*CureOption {
	var options []*CureOption

	allSpells := registry.GetAllSpells()
	for _, s := range allSpells {
		if s.Type != spell.Healing {
			continue
		}

		if !strings.HasPrefix(s.English, "Curaga") {
			continue
		}

		// Check MP requirement
		if int(s.MPCost) > availableMP {
			continue
		}

		// Check job level requirement
		canCast := false
		for job, level := range jobLevel {
			if requiredLevel, exists := s.LevelReq[job]; exists {
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
			SpellName:      s.English,
			Spell:          s,
			HealAmount:     s.HealAmount,
			MPCost:         int(s.MPCost),
			CastTime:       s.CastTime,
			Efficiency:     float64(s.HealAmount) / float64(s.MPCost),
			TimeEfficiency: float64(s.HealAmount) / float64(s.CastTime),
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
		return nil
	}

	var bestOption *CureOption
	bestScore := -1.0

	for _, option := range options {
		var score float64
		var scoreBreakdown string
		_ = scoreBreakdown

		if prioritizeEfficiency {
			healCoverage := math.Min(float64(option.HealAmount)/float64(missingHP), 1.0)

			appropriatenessBonus := 1.0
			var bonusReason string

			if healCoverage >= 0.7 && healCoverage <= 1.2 {
				appropriatenessBonus = 1.5
				bonusReason = "good_coverage"
			} else if healCoverage < 0.5 {
				appropriatenessBonus = 0.3
				bonusReason = "insufficient_healing"
			} else if healCoverage > 2.0 {
				appropriatenessBonus = 0.6
				bonusReason = "excessive_overheal"
			} else {
				bonusReason = "standard"
			}

			score = option.Efficiency * appropriatenessBonus * (1.0 - option.OverhealWaste*0.3)
			scoreBreakdown = fmt.Sprintf("efficiency_mode: eff=%.2f * bonus=%.2f (%s) * waste_penalty=%.2f",
				option.Efficiency, appropriatenessBonus, bonusReason, (1.0 - option.OverhealWaste*0.3))
		} else {
			healCoverage := float64(option.HealAmount) / float64(missingHP)

			var healingScore float64
			var healingReason string

			if healCoverage >= 0.8 {
				healingScore = 1.0 + (healCoverage-0.8)*0.5
				healingReason = "good_coverage"
			} else if healCoverage >= 0.5 {
				healingScore = healCoverage * 1.2
				healingReason = "moderate_coverage"
			} else {
				healingScore = healCoverage * 0.2
				healingReason = "poor_coverage"
			}

			score = healingScore * option.TimeEfficiency * (1.0 - option.OverhealWaste*0.2)
			scoreBreakdown = fmt.Sprintf("emergency_mode: healing=%.2f (%s) * time_eff=%.2f * waste_penalty=%.2f",
				healingScore, healingReason, option.TimeEfficiency, (1.0 - option.OverhealWaste*0.2))
		}

		if score > bestScore {
			bestScore = score
			bestOption = option
		}
	}

	return bestOption
}

// SelectOptimalCuragaForMultipleTargets selects the best Curaga spell when multiple targets need healing
func (cs *CureSelector) selectBestCuragaOption(options []*CureOption, averageMissingHP int, membersCount int) *CureOption {
	if len(options) == 0 {
		return nil
	}

	var bestOption *CureOption
	bestScore := -1.0

	for _, option := range options {
		totalHealingPotential := option.HealAmount * membersCount
		totalMPEfficiency := float64(totalHealingPotential) / float64(option.MPCost)

		var overhealWaste float64
		if option.HealAmount > averageMissingHP {
			overhealWaste = float64(option.HealAmount-averageMissingHP) / float64(option.HealAmount)
		}
		score := totalMPEfficiency * (1.0 - overhealWaste*0.4)

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
