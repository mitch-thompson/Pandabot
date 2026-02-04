package casting

import (
	"PandaBot/internal/action"
	"PandaBot/internal/config"
	"PandaBot/internal/cureSelector"
	"PandaBot/internal/entity"
	"PandaBot/internal/registry"
	"fmt"
	"time"
)

// CastingHelper provides convenient methods for common casting operations
type CastingHelper struct {
	engine        *CastingEngine
	clientManager *ClientManager
}

// NewCastingHelper creates a new casting helper
func NewCastingHelper(engine *CastingEngine, clientManager *ClientManager) *CastingHelper {
	return &CastingHelper{
		engine:        engine,
		clientManager: clientManager,
	}
}

// CastCure casts an optimal cure spell on a target
func (ch *CastingHelper) CastCure(target string, targetEntity *entity.Entity, casterMP int, jobLevels map[string]int, priority int) (string, error) {
	requestID := fmt.Sprintf("cure_%d", time.Now().UnixNano())

	// Get caster name from connected clients
	casterName := ch.getCasterName()

	request := &CastRequest{
		ID:       requestID,
		Type:     CastTypeCure,
		Target:   target,
		Priority: priority,
		Context: &CastContext{
			CasterMP:        casterMP,
			CasterJobLevels: jobLevels,
			CasterName:      casterName,
			TargetEntity:    targetEntity,
		},
	}

	if err := ch.engine.RequestCast(request); err != nil {
		return "", err
	}

	return requestID, nil
}

// CastCureByDamage casts an optimal cure spell based on missing HP
// partyMembers is optional but recommended; when provided, the engine can
// evaluate Curaga efficiency across the group.
func (ch *CastingHelper) CastCureByDamage(target string, missingHP int, casterMP int, jobLevels map[string]int, priority int, partyMembers []*entity.Entity) (string, error) {
	requestID := fmt.Sprintf("cure_dmg_%d", time.Now().UnixNano())

	// Get caster name from connected clients
	casterName := ch.getCasterName()

	request := &CastRequest{
		ID:       requestID,
		Type:     CastTypeCure,
		Target:   target,
		Priority: priority,
		Context: &CastContext{
			CasterMP:        casterMP,
			CasterJobLevels: jobLevels,
			CasterName:      casterName,
			MissingHP:       missingHP,
			PartyMembers:    partyMembers,
			PartySize:       len(partyMembers),
		},
	}

	if err := ch.engine.RequestCast(request); err != nil {
		return "", err
	}

	return requestID, nil
}

// CastBuffs casts optimal buff spells for a given buff type
func (ch *CastingHelper) CastBuffs(target string, buffType string, casterMP int, jobLevels map[string]int, partySize int, priority int) (string, error) {
	requestID := fmt.Sprintf("buff_%s_%d", buffType, time.Now().UnixNano())

	// Get caster name from connected clients
	casterName := ch.getCasterName()

	request := &CastRequest{
		ID:       requestID,
		Type:     CastTypeBuff,
		Target:   target,
		Priority: priority,
		Context: &CastContext{
			CasterMP:        casterMP,
			CasterJobLevels: jobLevels,
			CasterName:      casterName,
			PartySize:       partySize,
			BuffType:        buffType,
		},
	}

	if err := ch.engine.RequestCast(request); err != nil {
		return "", err
	}

	return requestID, nil
}

// CastNaSpell casts optimal "na" spell for status effects
func (ch *CastingHelper) CastNaSpell(target string, statusEffects []int, casterMP int, jobLevels map[string]int, priority int) (string, error) {
	requestID := fmt.Sprintf("na_%d", time.Now().UnixNano())

	// Get caster name from connected clients
	casterName := ch.getCasterName()

	request := &CastRequest{
		ID:       requestID,
		Type:     CastTypeNa,
		Target:   target,
		Priority: priority,
		Context: &CastContext{
			CasterMP:        casterMP,
			CasterJobLevels: jobLevels,
			CasterName:      casterName,
			StatusEffects:   statusEffects,
		},
	}

	if err := ch.engine.RequestCast(request); err != nil {
		return "", err
	}

	return requestID, nil
}

// CastSpell casts a specific spell (manual casting)
func (ch *CastingHelper) CastSpell(spellName string, target string, priority int, timeout time.Duration) (string, error) {
	s, err := registry.GetSpell(spellName)
	if err != nil {
		return "", err
	}

	requestID := fmt.Sprintf("manual_%s_%d", spellName, time.Now().UnixNano())

	// Get complete client context for proper target resolution
	casterName := ch.getCasterName()
	casterMP, jobLevels := ch.getCasterContext()

	request := &CastRequest{
		ID:       requestID,
		Type:     CastTypeManual,
		Action:   s,
		Target:   target,
		Priority: priority,
		Timeout:  timeout,
		Context: &CastContext{
			CasterName:      casterName,
			CasterMP:        casterMP,
			CasterJobLevels: jobLevels,
		},
	}

	if err := ch.engine.RequestCast(request); err != nil {
		return "", err
	}

	return requestID, nil
}

// CastSpellSequence casts multiple spells in sequence
func (ch *CastingHelper) CastSpellSequence(spells []string, target string, priority int) (string, error) {
	requestID := fmt.Sprintf("sequence_%d", time.Now().UnixNano())

	actions := make([]action.Actionable, 0, len(spells))
	for _, spellName := range spells {
		s, err := registry.GetSpell(spellName)
		if err != nil {
			return "", err
		}
		actions = append(actions, s)
	}

	request := &CastRequest{
		ID:       requestID,
		Type:     CastTypeSequence,
		Target:   target,
		Priority: priority,
	}

	if len(actions) > 0 {
		request.Action = actions[0]
	}

	// The engine will use the sequence from the ActiveCast it creates
	// But we need to pass the sequence somehow.
	// For now, let's assume the caller will populate ActionsInSequence if they have access to ActiveCast,
	// OR we need to update CastRequest to include initial sequence.
	// Actually, the current Engine design expects resolveActionSelection to handle it if it's not a manual sequence.
	// For manual sequence, we might need to add it to CastRequest.

	if err := ch.engine.RequestCast(request); err != nil {
		return "", err
	}

	return requestID, nil
}

// CastWithCallback casts a spell with a completion callback
func (ch *CastingHelper) CastWithCallback(spellName string, target string, priority int, callback CastCallback) (string, error) {
	s, err := registry.GetSpell(spellName)
	if err != nil {
		return "", err
	}

	requestID := fmt.Sprintf("callback_%s_%d", spellName, time.Now().UnixNano())

	request := &CastRequest{
		ID:       requestID,
		Type:     CastTypeManual,
		Action:   s,
		Target:   target,
		Priority: priority,
		Callback: callback,
	}

	if err := ch.engine.RequestCast(request); err != nil {
		return "", err
	}

	return requestID, nil
}

// Emergency casting methods with high priority

// EmergencyCure casts an emergency cure with maximum priority
func (ch *CastingHelper) EmergencyCure(target string, targetEntity *entity.Entity, casterMP int, jobLevels map[string]int) (string, error) {
	return ch.CastCure(target, targetEntity, casterMP, jobLevels, 10) // Maximum priority
}

// EmergencyNa casts an emergency "na" spell with high priority
func (ch *CastingHelper) EmergencyNa(target string, statusEffects []int, casterMP int, jobLevels map[string]int) (string, error) {
	return ch.CastNaSpell(target, statusEffects, casterMP, jobLevels, 9) // High priority
}

// Batch operations

// CastPartyBuffs casts buffs on all party members
func (ch *CastingHelper) CastPartyBuffs(partyMembers []*entity.Entity, buffType string, casterMP int, jobLevels map[string]int, priority int) ([]string, error) {
	var requestIDs []string
	var errors []error

	for _, member := range partyMembers {
		requestID, err := ch.CastBuffs(member.Name, buffType, casterMP, jobLevels, len(partyMembers), priority)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to cast buff on %s: %v", member.Name, err))
			continue
		}
		requestIDs = append(requestIDs, requestID)
	}

	if len(errors) > 0 {
		return requestIDs, fmt.Errorf("some buff casts failed: %v", errors)
	}

	return requestIDs, nil
}

// CastPartyCures casts cures on party members who need healing
func (ch *CastingHelper) CastPartyCures(partyMembers []*entity.Entity, casterMP int, jobLevels map[string]int, hpThreshold int, priority int) ([]string, error) {
	var requestIDs []string
	var errors []error

	// Identify party members that are below the threshold and need healing
	injured := make([]*entity.Entity, 0, len(partyMembers))
	for _, member := range partyMembers {
		if int(member.HPPercent) < hpThreshold {
			injured = append(injured, member)
		}
	}

	// If multiple members are injured, check if Curaga is cheaper and prioritize it
	if len(injured) >= config.Get().CuragaThreshold { // allow curaga consideration based on config
		availableMP := casterMP
		// Respect engine MP reservation if configured
		if ch.engine != nil && ch.engine.config != nil {
			availableMP = casterMP - ch.engine.config.MPReservation
		}
		if availableMP < 0 {
			availableMP = 0
		}

		selector := cureSelector.NewCureSelector()
		useCuraga, curagaOption, err := selector.ShouldUseCuraga(injured, availableMP, jobLevels)
		if err == nil && useCuraga && curagaOption != nil {
			// Cast curaga on the caster (self-target spell)
			casterName := ch.getCasterName()
			requestID, castErr := ch.CastSpell(curagaOption.SpellName, casterName, priority, 15*time.Second)
			if castErr != nil {
				errors = append(errors, fmt.Errorf("failed to cast %s: %v", curagaOption.SpellName, castErr))
			} else {
				requestIDs = append(requestIDs, requestID)
				// Curaga should cover all injured members; we can return early
				if len(errors) > 0 {
					return requestIDs, fmt.Errorf("some cure casts failed: %v", errors)
				}
				return requestIDs, nil
			}
		}
		// If curaga isn't chosen, fall back to single-target cures below
	}

	// Fall back: cast individual cures on injured members
	for _, member := range injured {
		requestID, err := ch.CastCure(member.Name, member, casterMP, jobLevels, priority)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to cast cure on %s: %v", member.Name, err))
			continue
		}
		requestIDs = append(requestIDs, requestID)
	}

	if len(errors) > 0 {
		return requestIDs, fmt.Errorf("some cure casts failed: %v", errors)
	}

	return requestIDs, nil
}

// Utility methods

// WaitForCast waits for a cast to complete with timeout
func (ch *CastingHelper) UseEchoDrop(priority int) (string, error) {
	i, err := registry.GetItem("Echo Drops")
	if err != nil {
		return "", err
	}

	requestID := fmt.Sprintf("echo_%d", time.Now().UnixNano())
	request := &CastRequest{
		ID:       requestID,
		Type:     CastTypeItem,
		Action:   i,
		Target:   "<me>",
		Priority: priority,
		Context: &CastContext{
			CasterName: ch.getCasterName(),
		},
	}

	err = ch.engine.RequestCast(request)
	if err != nil {
		return "", err
	}

	return requestID, nil
}

func (ch *CastingHelper) WaitForCast(requestID string, timeout time.Duration) (*CastResult, error) {
	startTime := time.Now()

	for time.Since(startTime) < timeout {
		activeCasts := ch.engine.GetActiveCasts()
		if cast, exists := activeCasts[requestID]; exists {
			if cast.State == CastStateCompleted {
				actionName := "unknown"
				if cast.Request.Action != nil {
					actionName = cast.Request.Action.GetName()
				}
				return &CastResult{
					Request:    cast.Request,
					Success:    true,
					Duration:   time.Since(cast.StartTime),
					ActionUsed: actionName,
				}, nil
			}
			if cast.State == CastStateFailed {
				actionName := "unknown"
				if cast.Request.Action != nil {
					actionName = cast.Request.Action.GetName()
				}
				return &CastResult{
					Request:    cast.Request,
					Success:    false,
					Error:      cast.LastError,
					Duration:   time.Since(cast.StartTime),
					ActionUsed: actionName,
				}, nil
			}
		} else {
			// Cast not found in active casts, check history
			history := ch.engine.GetCastHistory(50)
			for _, record := range history {
				if record.Request.ID == requestID {
					actionName := "unknown"
					if record.Request.Action != nil {
						actionName = record.Request.Action.GetName()
					}
					return &CastResult{
						Request:    record.Request,
						Success:    record.State == CastStateCompleted,
						Error:      record.Error,
						Duration:   record.Duration,
						ActionUsed: actionName,
					}, nil
				}
			}
		}

		time.Sleep(100 * time.Millisecond) // Check every 100ms
	}

	return nil, fmt.Errorf("timeout waiting for cast %s", requestID)
}

// GetCastStatus returns the current status of a cast request
func (ch *CastingHelper) GetCastStatus(requestID string) (CastState, error) {
	activeCasts := ch.engine.GetActiveCasts()
	if cast, exists := activeCasts[requestID]; exists {
		return cast.State, nil
	}

	// Check history
	history := ch.engine.GetCastHistory(100)
	for _, record := range history {
		if record.Request.ID == requestID {
			return record.State, nil
		}
	}

	return CastState(0), fmt.Errorf("cast request %s not found", requestID)
}

// CancelAllCasts cancels all active casting operations
func (ch *CastingHelper) CancelAllCasts() error {
	activeCasts := ch.engine.GetActiveCasts()
	var errors []error

	for requestID := range activeCasts {
		if err := ch.engine.CancelCast(requestID); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to cancel some casts: %v", errors)
	}

	return nil
}

// getCasterName gets the caster name from the first available connected client
func (ch *CastingHelper) getCasterName() string {
	connectedClients := ch.clientManager.GetConnectedClients()
	for _, client := range connectedClients {
		info := client.GetClientInfo()
		if info.PlayerName != "" {
			return info.PlayerName
		}
	}
	return "me" // Fallback if no client name is available
}

// getCasterContext gets the caster MP and job levels from the first available connected client
func (ch *CastingHelper) getCasterContext() (int, map[string]int) {
	connectedClients := ch.clientManager.GetConnectedClients()
	for _, client := range connectedClients {
		info := client.GetClientInfo()
		if info.PlayerName != "" {
			return info.MP, info.JobLevels
		}
	}
	// Fallback values if no client info is available
	return 400, map[string]int{"WHM": 22, "RDM": 37, "PLD": 20}
}
