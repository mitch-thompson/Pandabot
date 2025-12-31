package triggerService

import (
	"log"

	"PandaBot/internal/casting"
	"PandaBot/internal/entityService"
	"PandaBot/internal/statusMonitor"
	"PandaBot/internal/textParser"
)

// TriggerService handles routing of trigger events to the casting system
type TriggerService struct {
	castingSystem   *casting.CastingServerIntegration
	entityService   *entityService.EntityService
	buffToStatusMap map[string]int
}

// NewTriggerService creates a new trigger service
func NewTriggerService(castingSystem *casting.CastingServerIntegration) *TriggerService {
	return &TriggerService{
		castingSystem:   castingSystem,
		entityService:   entityService.NewEntityService(),
		buffToStatusMap: statusMonitor.GetBuffToStatusMap(),
	}
}

// RouteTriggerEvents routes trigger events to the centralized casting system
func (ts *TriggerService) RouteTriggerEvents(triggerEvents []textParser.TriggerEvent, sm *statusMonitor.StatusMonitor) {
	// Get current party members from status monitor
	partyMembers := sm.GetAllPartyMembers()

	// Convert to entity format for casting system
	entityMembers := ts.entityService.ConvertPartyMembersToEntities(partyMembers)

	// Process each trigger event through centralized casting system
	for _, triggerEvent := range triggerEvents {
		// Handle "panda" control trigger (clear queue and buffs)
		if triggerEvent.TriggerType == "panda" {
			log.Printf("Executing clear command: clearing casting queue and tracked buffs")
			ts.castingSystem.GetCastingEngine().ClearQueue()
			sm.ClearDesiredBuffs()
			continue
		}

		// Register buff for monitoring if it's a known buff
		if statusID, ok := ts.buffToStatusMap[triggerEvent.TriggerType]; ok {
			// Some buffs are always self-buffs
			target := triggerEvent.Sender
			if triggerEvent.TriggerType == "light arts" || triggerEvent.TriggerType == "lightarts" ||
				triggerEvent.TriggerType == "dark arts" || triggerEvent.TriggerType == "darkarts" ||
				triggerEvent.TriggerType == "afflatus solace" || triggerEvent.TriggerType == "solace" ||
				triggerEvent.TriggerType == "afflatus misery" || triggerEvent.TriggerType == "misery" ||
				triggerEvent.TriggerType == "reraise" {
				target = "<me>"
			}
			sm.RegisterDesiredBuff(target, statusID, triggerEvent.TriggerType)
		}

		// Special handling for elemental buff sequences (firebuffs, etc.)
		elementalMapping := statusMonitor.GetElementalBarStatusMapping()
		if mapping, ok := elementalMapping[triggerEvent.TriggerType]; ok {
			// Register Protect and Shell for monitoring
			sm.RegisterDesiredBuff(triggerEvent.Sender, 40, "protect")
			sm.RegisterDesiredBuff(triggerEvent.Sender, 41, "shell")

			// Register the specific Bar spell - Bar spells always target the caster
			sm.RegisterDesiredBuff("<me>", mapping.StatusID, mapping.SpellName)

			// If it's a WHM-enabled buff sequence, also register WHM prep buffs
			// These are always self-buffs, so they should target "<me>"
			sm.RegisterDesiredBuff("<me>", 358, "light arts")
			sm.RegisterDesiredBuff("<me>", 417, "afflatus solace")
			sm.RegisterDesiredBuff("<me>", 113, "reraise")
			sm.RegisterDesiredBuff("<me>", 272, "auspice")
		}

		requestIDs := ts.castingSystem.ProcessTriggerEvent(
			triggerEvent.TriggerType,
			triggerEvent.Sender,
			triggerEvent.Priority,
			entityMembers,
		)

		if len(requestIDs) > 0 {
			log.Printf("Routed trigger event %s from %s to casting system, generated %d requests",
				triggerEvent.TriggerType, triggerEvent.Sender, len(requestIDs))
		}
	}
}
