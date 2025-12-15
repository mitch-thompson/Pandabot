package casting

import (
	"fmt"
	"log"
	"strings"
	"time"

	"PandaBot/internal/entity"
)

// TriggerProcessor handles processing of trigger events and converts them to casting requests
type TriggerProcessor struct {
	engine *CastingEngine
}

// NewTriggerProcessor creates a new trigger processor
func NewTriggerProcessor(engine *CastingEngine) *TriggerProcessor {
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
	
	switch {
	// Status removal triggers
	case triggerType == "stoned":
		requestID, err := tp.processNaTrigger("Stona", sender, priority, context)
		if err != nil {
			return nil, fmt.Errorf("failed to process stoned trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)
		
	case triggerType == "paralyzed":
		requestID, err := tp.processNaTrigger("Paralyna", sender, priority, context)
		if err != nil {
			return nil, fmt.Errorf("failed to process paralyzed trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)
		
	case triggerType == "silenced":
		requestID, err := tp.processNaTrigger("Silena", sender, priority, context)
		if err != nil {
			return nil, fmt.Errorf("failed to process silenced trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)
		
	case triggerType == "poisoned":
		requestID, err := tp.processNaTrigger("Poisona", sender, priority, context)
		if err != nil {
			return nil, fmt.Errorf("failed to process poisoned trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)
		
	case triggerType == "blinded":
		requestID, err := tp.processNaTrigger("Blindna", sender, priority, context)
		if err != nil {
			return nil, fmt.Errorf("failed to process blinded trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)
		
	// Healing triggers
	case triggerType == "heal" || triggerType == "cure" || triggerType == "help":
		requestID, err := tp.processCureTrigger(sender, priority, context)
		if err != nil {
			return nil, fmt.Errorf("failed to process heal trigger: %v", err)
		}
		requestIDs = append(requestIDs, requestID)
		
	// Buff triggers - these target the caster, not the sender
	case strings.HasSuffix(triggerType, "buffs"):
		buffType := triggerType // firebuffs, waterbuffs, etc.
		requestID, err := tp.processBuffTrigger(buffType, casterName, priority, context)
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
func (tp *TriggerProcessor) processNaTrigger(spellName string, target string, priority int, context *CastContext) (string, error) {
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
		return tp.castManualSpell(spellName, target, priority, context)
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
	requestID := fmt.Sprintf("na_%d", time.Now().UnixNano())
	request := &CastRequest{
		ID:       requestID,
		Type:     CastTypeManual, // Use manual type with specific spell
		SpellName: spellName,
		Target:   target,
		Priority: priority,
		Context:  context,
	}
	
	err := tp.engine.RequestCast(request)
	if err != nil {
		return "", fmt.Errorf("failed to request na spell cast: %v", err)
	}
	
	return requestID, nil
}

// processCureTrigger processes a healing trigger
func (tp *TriggerProcessor) processCureTrigger(target string, priority int, context *CastContext) (string, error) {
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
		return tp.castManualSpell("Cure", target, priority, context)
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
	requestID := fmt.Sprintf("cure_%d", time.Now().UnixNano())
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
func (tp *TriggerProcessor) processBuffTrigger(buffType string, target string, priority int, context *CastContext) (string, error) {
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
	requestID := fmt.Sprintf("buff_%d", time.Now().UnixNano())
	request := &CastRequest{
		ID:       requestID,
		Type:     CastTypeBuff, // Let engine select optimal buff sequence
		Target:   buffTarget, // Buff spells target the caster
		Priority: priority,
		Context:  context,
	}
	
	err := tp.engine.RequestCast(request)
	if err != nil {
		return "", fmt.Errorf("failed to request buff cast: %v", err)
	}
	
	return requestID, nil
}

// castManualSpell casts a specific spell manually
func (tp *TriggerProcessor) castManualSpell(spellName string, target string, priority int, context *CastContext) (string, error) {
	requestID := fmt.Sprintf("manual_%d", time.Now().UnixNano())
	request := &CastRequest{
		ID:        requestID,
		Type:      CastTypeManual,
		SpellName: spellName,
		Target:    target,
		Priority:  priority,
		Context:   context,
	}
	
	err := tp.engine.RequestCast(request)
	if err != nil {
		return "", fmt.Errorf("failed to request manual spell cast: %v", err)
	}
	
	return requestID, nil
}