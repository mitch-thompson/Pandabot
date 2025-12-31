package autoActionService

import (
	"log"

	"PandaBot/internal/casting"
	"PandaBot/internal/entity"
	"PandaBot/internal/statusMonitor"
)

// AutoActionService handles automatic actions based on party status
type AutoActionService struct {
	castingSystem *casting.CastingServerIntegration
}

// NewAutoActionService creates a new auto action service
func NewAutoActionService(castingSystem *casting.CastingServerIntegration) *AutoActionService {
	return &AutoActionService{
		castingSystem: castingSystem,
	}
}

// ProcessAutomaticActions checks for actions based on current party status
func (aas *AutoActionService) ProcessAutomaticActions(statusMonitor *statusMonitor.StatusMonitor) {
	actions := statusMonitor.CheckForActions()

	if len(actions) == 0 {
		return
	}

	log.Printf("Automatic actions triggered: %d", len(actions))

	// Use centralized casting system for automatic actions
	castingHelper := aas.castingSystem.GetCastingHelper()

	for _, action := range actions {
		switch action.Type {
		case "cure":
			// Use centralized cure casting
			member, exists := statusMonitor.GetPartyMember(action.Target)
			if !exists {
				continue
			}

			// Calculate missing HP using actual values if available
			var missingHP int
			if member.HPMax > 0 && member.HPActual >= 0 {
				missingHP = member.HPMax - member.HPActual
				log.Printf("[AUTO CURE DEBUG] Using actual HP values: %d/%d HP, missing %d",
					member.HPActual, member.HPMax, missingHP)
			} else {
				// Fallback to percentage-based calculation
				missingHP = 100 - member.HPPercent
				log.Printf("[AUTO CURE DEBUG] Using percentage fallback: %d%% HP, missing %d%%",
					member.HPPercent, missingHP)
			}

			// Get client info for casting context
			connectedClients := aas.castingSystem.GetClientManager().GetConnectedClients()
			if len(connectedClients) == 0 {
				log.Printf("No connected clients available for automatic cure casting")
				continue
			}

			// Use first available client's info
			var clientInfo *casting.ClientInfo
			for _, client := range connectedClients {
				clientInfo = client.GetClientInfo()
				break
			}

			if clientInfo == nil {
				continue
			}

			// Build party member context so Curaga efficiency can be evaluated
			partyEntities := buildPartyEntities(statusMonitor)

			requestID, err := castingHelper.CastCureByDamage(
				action.Target,
				missingHP,
				clientInfo.MP,
				clientInfo.JobLevels,
				action.Priority,
				partyEntities,
			)

			if err != nil {
				log.Printf("Failed to queue automatic cure for %s: %v", action.Target, err)
			} else {
				log.Printf("Queued automatic cure for %s (request ID: %s)", action.Target, requestID)
			}

		case "na_spell":
			// Use centralized na spell casting
			member, exists := statusMonitor.GetPartyMember(action.Target)
			if !exists {
				continue
			}

			// Get client info for casting context
			connectedClients := aas.castingSystem.GetClientManager().GetConnectedClients()
			if len(connectedClients) == 0 {
				log.Printf("No connected clients available for automatic na spell casting")
				continue
			}

			// Use first available client's info
			var clientInfo *casting.ClientInfo
			for _, client := range connectedClients {
				clientInfo = client.GetClientInfo()
				break
			}

			if clientInfo == nil {
				continue
			}

			// Convert member status effects to int slice
			var statusEffects []int
			for _, effect := range member.StatusIDs {
				statusEffects = append(statusEffects, effect)
			}

			requestID, err := castingHelper.CastNaSpell(
				action.Target,
				statusEffects,
				clientInfo.MP,
				clientInfo.JobLevels,
				action.Priority,
			)

			if err != nil {
				log.Printf("Failed to queue automatic na spell for %s: %v", action.Target, err)
			} else {
				log.Printf("Queued automatic na spell for %s (request ID: %s)", action.Target, requestID)
			}

		case "echo_drop":
			requestID, err := castingHelper.UseEchoDrop(action.Priority)
			if err != nil {
				log.Printf("Failed to use echo drop: %v", err)
			} else {
				log.Printf("Queued echo drop usage (request ID: %s)", requestID)
			}

		case "manual_spell":
			// For desired buffs that are missing, add back into the queue
			partyEntities := buildPartyEntities(statusMonitor)
			requestIDs := aas.castingSystem.ProcessTriggerEvent(
				action.Spell,
				action.Target,
				action.Priority,
				partyEntities,
			)
			if len(requestIDs) == 0 {
				log.Printf("Failed to queue missing buff %s for %s", action.Spell, action.Target)
			} else {
				log.Printf("Queued missing buff %s for %s (request IDs: %v)", action.Spell, action.Target, requestIDs)
			}
		}
	}
}

// buildPartyEntities converts the status monitor's party view into entity.Entity list
func buildPartyEntities(sm *statusMonitor.StatusMonitor) []*entity.Entity {
	partyMap := sm.GetAllPartyMembers()
	entities := make([]*entity.Entity, 0, len(partyMap))
	for name, pm := range partyMap {
		e := &entity.Entity{
			Name:      name,
			HPPercent: uint8(pm.HPPercent),
			MPPercent: uint8(pm.MPPercent),
			HPcurrent: uint32(pm.HPActual),
			HPMax:     uint32(pm.HPMax),
			Zone:      uint16(pm.Zone),
			// Job fields are optional for Curaga decision; leave empty if unknown
		}
		entities = append(entities, e)
	}
	return entities
}
