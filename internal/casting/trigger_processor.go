package casting

import (
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"PandaBot/internal/entity"
	"PandaBot/internal/registry"
)

var globalRequestSeq uint64

type Engine interface {
	RequestCast(request *CastRequest) error
}

// TriggerProcessor handles processing of trigger events and converts them to casting requests
type TriggerProcessor struct {
	engine Engine
}

// NewTriggerProcessor creates a new trigger processor
func NewTriggerProcessor(engine Engine) *TriggerProcessor {
	return &TriggerProcessor{
		engine: engine,
	}
}

// ProcessTriggerEvent processes a trigger event and generates appropriate casting requests
func (tp *TriggerProcessor) ProcessTriggerEvent(triggerType string, sender string, priority int, casterName string, casterMP int, casterJobLevels map[string]int, partyMembers []*entity.Entity) ([]string, error) {
	var requestIDs []string

	// Create casting context
	context := &CastContext{
		CasterMP:        casterMP,
		CasterJobLevels: casterJobLevels,
		CasterName:      casterName,
		PartyMembers:    partyMembers,
		PartySize:       len(partyMembers),
	}

	// Use a base timestamp for all requests in this call, but we'll add a sequence number to ensure uniqueness
	baseNano := time.Now().UnixNano() + int64(atomic.AddUint64(&globalRequestSeq, 1000))
	getUniqueNano := func(seq int) int64 {
		return baseNano + int64(seq)
	}
	seq := 0

	switch {
	// Status removal triggers
	case triggerType == "stoned":
		requestID, err := tp.processNaTrigger("Stona", sender, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process stoned trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "paralyzed":
		requestID, err := tp.processNaTrigger("Paralyna", sender, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process paralyzed trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "silenced" || triggerType == "silence" || triggerType == "silena":
		requestID, err := tp.processNaTrigger("Silena", sender, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process silenced trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "poisoned":
		requestID, err := tp.processNaTrigger("Poisona", sender, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process poisoned trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "blinded":
		requestID, err := tp.processNaTrigger("Blindna", sender, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process blinded trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "erase":
		requestID, err := tp.processNaTrigger("Erase", sender, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process erase trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "cursna" || triggerType == "cursed" || triggerType == "doom":
		requestID, err := tp.processNaTrigger("Cursna", sender, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process cursna trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "devotion":
		requestID, err := tp.processAbilityTrigger("Devotion", sender, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process devotion trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "viruna" || triggerType == "diseased" || triggerType == "plagued":
		requestID, err := tp.processNaTrigger("Viruna", sender, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process viruna trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	// Healing triggers
	case triggerType == "heal" || triggerType == "cure" || triggerType == "help":
		requestID, err := tp.processCureTrigger(sender, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process heal trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "protect":
		requestID, err := tp.processProtectTrigger(sender, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process protect trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "shell":
		requestID, err := tp.processShellTrigger(sender, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process shell trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "haste":
		requestID, err := tp.castManualSpell("Haste", sender, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process haste trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "regen":
		log.Printf("Regen received from %s", sender)
		requestID, err := tp.processRegenTrigger(sender, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process regen trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "auspice":
		requestID, err := tp.castManualSpell("Auspice", casterName, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process auspice trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "reraise":
		requestID, err := tp.processReraiseTrigger(casterName, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process reraise trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "light arts" || triggerType == "lightarts":
		requestID, err := tp.castManualSpell("Light Arts", casterName, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process light arts trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "dark arts" || triggerType == "darkarts":
		requestID, err := tp.castManualSpell("Dark Arts", casterName, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process dark arts trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "afflatus solace" || triggerType == "solace":
		requestID, err := tp.castManualSpell("Afflatus Solace", casterName, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process afflatus solace trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "afflatus misery" || triggerType == "misery":
		requestID, err := tp.castManualSpell("Afflatus Misery", casterName, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process afflatus misery trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "barfire":
		requestID, err := tp.castManualSpell("Barfira", casterName, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process barfire trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "barice":
		requestID, err := tp.castManualSpell("Barblizzara", casterName, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process barice trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "barwind":
		requestID, err := tp.castManualSpell("Baraera", casterName, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process barwind trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "barearth":
		requestID, err := tp.castManualSpell("Barstonra", casterName, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process barearth trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "barlightning":
		requestID, err := tp.castManualSpell("Barthundra", casterName, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process barlightning trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	case triggerType == "barwater":
		requestID, err := tp.castManualSpell("Barwatera", casterName, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process barwater trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)

	// Control triggers
	case triggerType == "panda":
		log.Printf("Panda control trigger received from %s", sender)
		// No request ID generated for internal control command
		return nil, nil

	// Buff triggers - these target the caster, not the sender
	case strings.HasSuffix(triggerType, "buffs"):
		buffType := triggerType // firebuffs, waterbuffs, etc.
		requestID, err := tp.processBuffTrigger(buffType, casterName, priority, context, getUniqueNano(seq))
		seq++
		if err != nil {
			return nil, fmt.Errorf("failed to process buff trigger %s: %v", buffType, err)
		}
		requestIDs = append(requestIDs, requestID)

	default:
		return nil, fmt.Errorf("unknown trigger type: %s", triggerType)
	}

	return requestIDs, nil
}

// processNaTrigger processes a status removal trigger
func (tp *TriggerProcessor) processNaTrigger(spellName string, target string, priority int, context *CastContext, timestamp int64) (string, error) {
	// Find the target entity to get their status effects
	var targetEntity *entity.Entity
	for _, member := range context.PartyMembers {
		if member.Name == target {
			targetEntity = member
			break
		}
	}

	if targetEntity == nil {
		// If we can't find the entity, still try to cast the specific spell
		log.Printf("Target entity %s not found, casting %s anyway", target, spellName)
		return tp.castManualSpell(spellName, target, priority, context, timestamp)
	}

	// Update context with target information
	context.TargetEntity = targetEntity

	// Convert Buffs array to status effects slice
	var statusEffects []int
	for _, buffID := range targetEntity.Buffs {
		if buffID != 0 {
			statusEffects = append(statusEffects, int(buffID))
		}
	}
	context.StatusEffects = statusEffects

	// Create casting request for na spell
	requestID := fmt.Sprintf("na_%d", timestamp)
	s, err := registry.GetSpell(spellName)
	if err != nil {
		return "", err
	}

	request := &CastRequest{
		ID:       requestID,
		Type:     CastTypeManual, // Use manual type with specific spell
		Action:   s,
		Target:   target,
		Priority: priority,
		Context:  context,
	}

	err = tp.engine.RequestCast(request)
	if err != nil {
		return "", fmt.Errorf("failed to request na spell cast: %v", err)
	}

	return requestID, nil
}

// processAbilityTrigger processes a job ability trigger
func (tp *TriggerProcessor) processAbilityTrigger(abilityName string, target string, priority int, context *CastContext, timestamp int64) (string, error) {
	// Find the target entity
	var targetEntity *entity.Entity
	for _, member := range context.PartyMembers {
		if member.Name == target {
			targetEntity = member
			break
		}
	}

	if targetEntity != nil {
		context.TargetEntity = targetEntity
	}

	// Create casting request for ability
	requestID := fmt.Sprintf("ja_%d", timestamp)
	a, err := registry.GetAbility(abilityName)
	if err != nil {
		return "", err
	}

	request := &CastRequest{
		ID:       requestID,
		Type:     CastTypeManual,
		Action:   a,
		Target:   target,
		Priority: priority,
		Context:  context,
	}

	err = tp.engine.RequestCast(request)
	if err != nil {
		return "", fmt.Errorf("failed to request ability cast: %v", err)
	}

	return requestID, nil
}

// processCureTrigger processes a healing trigger
func (tp *TriggerProcessor) processCureTrigger(target string, priority int, context *CastContext, timestamp int64) (string, error) {
	log.Printf("[TRIGGER DEBUG] Processing cure trigger for target: %s, priority: %d", target, priority)

	// Find the target entity to determine cure needs
	var targetEntity *entity.Entity
	for _, member := range context.PartyMembers {
		if member.Name == target {
			targetEntity = member
			break
		}
	}

	if targetEntity == nil {
		// If we can't find the entity, cast a basic cure
		log.Printf("[TRIGGER DEBUG] Target entity %s not found, casting basic Cure", target)
		return tp.castManualSpell("Cure", target, priority, context, timestamp)
	}

	log.Printf("[TRIGGER DEBUG] Found target entity: %s, HP: %d/%d (%d%%), Job: %s Level: %d",
		targetEntity.Name, targetEntity.HPcurrent, targetEntity.HPMax, targetEntity.HPPercent,
		targetEntity.Job, targetEntity.JobLevel)

	// Update context with target information
	context.TargetEntity = targetEntity

	// Calculate actual missing HP using both actual values and percentage fallback
	var missingHP int
	var calculationMethod string

	if targetEntity.HPMax > 0 && targetEntity.HPcurrent <= targetEntity.HPMax {
		// Use actual HP values when available (preferred)
		missingHP = int(targetEntity.HPMax - targetEntity.HPcurrent)
		calculationMethod = "actual_values"
		log.Printf("[TRIGGER DEBUG] Using actual HP values: %d - %d = %d missing HP",
			targetEntity.HPMax, targetEntity.HPcurrent, missingHP)
	} else {
		// Fallback to percentage-based calculation with realistic HP estimate
		missingPercent := 100 - int(targetEntity.HPPercent)
		estimatedMaxHP := 1000 // Default estimate
		calculationMethod = "percentage_estimate"

		log.Printf("[TRIGGER DEBUG] Actual HP values not available, using percentage: %d%% HP, missing %d%%",
			targetEntity.HPPercent, missingPercent)

		// Better HP estimate based on job level if available
		if targetEntity.JobLevel > 0 {
			baseHP := 500 + (int(targetEntity.JobLevel) * 18)
			switch targetEntity.Job {
			case "WAR", "PLD", "DRK", "MNK":
				estimatedMaxHP = int(float64(baseHP) * 1.2)
			case "WHM", "BLM", "RDM", "SMN", "SCH":
				estimatedMaxHP = int(float64(baseHP) * 0.9)
			default:
				estimatedMaxHP = baseHP
			}

			// Reasonable bounds
			if estimatedMaxHP < 200 {
				estimatedMaxHP = 200
			}
			if estimatedMaxHP > 2000 {
				estimatedMaxHP = 2000
			}

			log.Printf("[TRIGGER DEBUG] Estimated max HP for %s %d: %d (base: %d)",
				targetEntity.Job, targetEntity.JobLevel, estimatedMaxHP, baseHP)
		}

		missingHP = (missingPercent * estimatedMaxHP) / 100
		log.Printf("[TRIGGER DEBUG] Calculated missing HP: (%d%% * %d) / 100 = %d",
			missingPercent, estimatedMaxHP, missingHP)
	}

	context.MissingHP = missingHP
	log.Printf("[TRIGGER DEBUG] Final missing HP calculation: %d (method: %s)", missingHP, calculationMethod)

	// Create casting request for cure spell (let engine select optimal cure)
	requestID := fmt.Sprintf("cure_%d", timestamp)
	request := &CastRequest{
		ID:       requestID,
		Type:     CastTypeCure, // Let engine select optimal cure level
		Target:   target,
		Priority: priority,
		Context:  context,
	}

	err := tp.engine.RequestCast(request)
	if err != nil {
		return "", fmt.Errorf("failed to request cure cast: %v", err)
	}

	return requestID, nil
}

// processBuffTrigger processes a buff trigger
func (tp *TriggerProcessor) processBuffTrigger(buffType string, target string, priority int, context *CastContext, timestamp int64) (string, error) {
	// Update context with buff information
	context.BuffType = buffType

	log.Printf("[TRIGGER DEBUG] processBuffTrigger: buffType=%s, casterMP=%d, jobLevels=%v, partySize=%d",
		buffType, context.CasterMP, context.CasterJobLevels, context.PartySize)

	// For buff spells, the target should always be the caster, not the original sender
	buffTarget := context.CasterName
	if buffTarget == "" {
		return "", fmt.Errorf("caster name is required for buff spells but was empty")
	}

	// Create casting request for buff sequence
	requestID := fmt.Sprintf("buff_%d", timestamp)
	request := &CastRequest{
		ID:       requestID,
		Type:     CastTypeBuff, // Let engine select optimal buff sequence
		Target:   buffTarget,   // Buff spells target the caster
		Priority: priority,
		Context:  context,
	}

	err := tp.engine.RequestCast(request)
	if err != nil {
		return "", fmt.Errorf("failed to request buff cast: %v", err)
	}

	// Requirement: If the caster is a WHM, also prepare them
	if level, exists := context.CasterJobLevels["WHM"]; exists && level > 0 {
		log.Printf("[TRIGGER DEBUG] Caster is WHM, adding WHM preparation sequence")
		whmPrepRequest := &CastRequest{
			ID:       fmt.Sprintf("whmprep_%d", timestamp),
			Type:     CastTypeWhmPrep,
			Target:   context.CasterName,
			Priority: priority,
			Context:  context,
		}
		err = tp.engine.RequestCast(whmPrepRequest)
		if err != nil {
			log.Printf("[TRIGGER DEBUG] Failed to request WHM prep cast: %v", err)
		}
	}

	return requestID, nil
}

// castManualSpell casts a specific spell manually
func (tp *TriggerProcessor) castManualSpell(spellName string, target string, priority int, context *CastContext, timestamp int64) (string, error) {
	s, err := registry.GetSpell(spellName)
	if err != nil {
		return "", err
	}

	requestID := fmt.Sprintf("manual_%d", timestamp)
	request := &CastRequest{
		ID:       requestID,
		Type:     CastTypeManual,
		Action:   s,
		Target:   target,
		Priority: priority,
		Context:  context,
	}

	err = tp.engine.RequestCast(request)
	if err != nil {
		return "", fmt.Errorf("failed to request manual spell cast: %v", err)
	}

	return requestID, nil
}

// processProtectTrigger processes a protect trigger
func (tp *TriggerProcessor) processProtectTrigger(target string, priority int, context *CastContext, timestamp int64) (string, error) {
	requestID := fmt.Sprintf("protect_%d", timestamp)
	request := &CastRequest{
		ID:       requestID,
		Type:     CastTypeProtect, // Let engine select optimal Protect level
		Target:   target,
		Priority: priority,
		Context:  context,
	}

	err := tp.engine.RequestCast(request)
	if err != nil {
		return "", fmt.Errorf("failed to request protect cast: %v", err)
	}

	return requestID, nil
}

// processReraiseTrigger processes a reraise trigger
func (tp *TriggerProcessor) processReraiseTrigger(target string, priority int, context *CastContext, timestamp int64) (string, error) {
	requestID := fmt.Sprintf("reraise_%d", timestamp)
	request := &CastRequest{
		ID:       requestID,
		Type:     CastTypeReraise, // Let engine select optimal Reraise level
		Target:   target,
		Priority: priority,
		Context:  context,
	}

	err := tp.engine.RequestCast(request)
	if err != nil {
		return "", fmt.Errorf("failed to request reraise cast: %v", err)
	}

	return requestID, nil
}

// processShellTrigger processes a shell trigger
func (tp *TriggerProcessor) processShellTrigger(target string, priority int, context *CastContext, timestamp int64) (string, error) {
	requestID := fmt.Sprintf("shell_%d", timestamp)
	request := &CastRequest{
		ID:       requestID,
		Type:     CastTypeShell, // Let engine select optimal Shell level
		Target:   target,
		Priority: priority,
		Context:  context,
	}

	err := tp.engine.RequestCast(request)
	if err != nil {
		return "", fmt.Errorf("failed to request shell cast: %v", err)
	}

	return requestID, nil
}

// processRegenTrigger processes a regen trigger
func (tp *TriggerProcessor) processRegenTrigger(target string, priority int, context *CastContext, timestamp int64) (string, error) {
	requestID := fmt.Sprintf("regen_%d", timestamp)
	request := &CastRequest{
		ID:       requestID,
		Type:     CastTypeRegen, // Let engine select optimal Regen level
		Target:   target,
		Priority: priority,
		Context:  context,
	}

	err := tp.engine.RequestCast(request)
	if err != nil {
		return "", fmt.Errorf("failed to request regen cast: %v", err)
	}

	return requestID, nil
}
