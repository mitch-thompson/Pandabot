package triggerService

import (
	"log"
	"time"

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
func (ts *TriggerService) RouteTriggerEvents(triggerEvents []textParser.TriggerEvent, sm *statusMonitor.StatusMonitor, disableCures bool, plSource string, plTarget string) {
	// Get current party members from status monitor
	partyMembers := sm.GetAllPartyMembers()

	// Convert to entity format for casting system
	entityMembers := ts.entityService.ConvertPartyMembersToEntities(partyMembers)

	// Determine if PL mode is active for trigger processing
	isPL := false
	if plTarget != "" && plSource != "" {
		isPL = true
	}

	for _, triggerEvent := range triggerEvents {
		// Handle "panda" and "panda clear" control triggers
		if triggerEvent.TriggerType == "panda" || triggerEvent.TriggerType == "panda clear" {
			if triggerEvent.TriggerType == "panda clear" {
				if triggerEvent.Arg == "" {
					log.Printf("Executing clear command: clearing all tracked buffs")
					sm.ClearDesiredBuffs()
				} else {
					// Try to clear by player name first
					if _, exists := sm.GetPartyMember(triggerEvent.Arg); exists {
						log.Printf("Executing clear command for player: %s", triggerEvent.Arg)
						sm.ClearDesiredBuff(triggerEvent.Arg)
					} else {
						// Otherwise try to clear by spell name
						log.Printf("Executing clear command for spell: %s", triggerEvent.Arg)
						sm.ClearDesiredBuffBySpell(triggerEvent.Arg)
					}
				}
			} else {
				log.Printf("Executing clear command: clearing casting queue and tracked buffs")
				ts.castingSystem.GetCastingEngine().ClearQueue()
				sm.ClearDesiredBuffs()
			}
			continue
		}

		// If cures are disabled, skip healing and status-removal triggers entirely
		if disableCures {
			// healing-related triggers to ignore when disabled
			switch triggerEvent.TriggerType {
			case "heal", "cure", "help", "erase", "cursna", "viruna", "doom",
				"stoned", "paralyzed", "silenced", "poisoned", "blinded", "cursed", "diseased", "plagued",
				"devotion":
				log.Printf("DisableCures active: ignoring trigger event %s from %s", triggerEvent.TriggerType, triggerEvent.Sender)
				continue
			}
		}

		// Register buff for monitoring if it's a known buff
		if statusID, ok := ts.buffToStatusMap[triggerEvent.TriggerType]; ok {
			// Some buffs are always self-buffs
			target := triggerEvent.Sender
			if triggerEvent.Arg != "" {
				target = triggerEvent.Arg
			}
			priority := 50 // Default priority
			if triggerEvent.TriggerType == "light arts" || triggerEvent.TriggerType == "lightarts" ||
				triggerEvent.TriggerType == "dark arts" || triggerEvent.TriggerType == "darkarts" ||
				triggerEvent.TriggerType == "afflatus solace" || triggerEvent.TriggerType == "solace" ||
				triggerEvent.TriggerType == "afflatus misery" || triggerEvent.TriggerType == "misery" {
				priority = 80
			} else if triggerEvent.TriggerType == "reraise" {
				priority = 90
			} else if triggerEvent.TriggerType == "regen" {
				priority = 40 // Regen usually lower priority than basic buffs
			} else if triggerEvent.TriggerType == "refresh" {
				priority = 60
			}
			sm.RegisterDesiredBuff(target, statusID, triggerEvent.TriggerType, priority, time.Time{})
		}

		// Special handling for elemental buff sequences (firebuffs, etc.)
		elementalMapping := statusMonitor.GetElementalBarStatusMapping()
		if mapping, ok := elementalMapping[triggerEvent.TriggerType]; ok {
			// Register Protect and Shell for monitoring for EVERYONE in the party
			// because they resolve to Protectra/Shellra which are AoE.
			// Even if they resolve to single target Protect/Shell, we want to maintain them on the sender.
			// But for AoE spells, it's better to register them for everyone.
			for _, member := range partyMembers {
				sm.RegisterDesiredBuff(member.Name, 40, "protect", 60, time.Time{})
				sm.RegisterDesiredBuff(member.Name, 41, "shell", 60, time.Time{})
			}

			// Register the specific Bar spell - Bar spells always target the caster
			sm.RegisterDesiredBuff(triggerEvent.Sender, mapping.StatusID, mapping.SpellName, 70, time.Time{})

			// If it's a WHM-enabled buff sequence, also register WHM prep buffs
			// These are always self-buffs, so they should target the sender (who requested the buffs)
			// autoActionService will handle redirecting these to <me> for monitoring.
			sm.RegisterDesiredBuff(triggerEvent.Sender, 358, "Light Arts", 80, time.Time{})
			sm.RegisterDesiredBuff(triggerEvent.Sender, 417, "Afflatus Solace", 80, time.Time{})
			sm.RegisterDesiredBuff(triggerEvent.Sender, 113, "reraise", 90, time.Time{})
			sm.RegisterDesiredBuff(triggerEvent.Sender, 272, "Auspice", 75, time.Time{})
		}

		requestIDs := ts.castingSystem.ProcessTriggerEvent(
			triggerEvent.TriggerType,
			triggerEvent.Sender,
			triggerEvent.Arg,
			triggerEvent.Priority,
			entityMembers,
			isPL,
		)

		if len(requestIDs) > 0 {
			log.Printf("Routed trigger event %s from %s to casting system, generated %d requests",
				triggerEvent.TriggerType, triggerEvent.Sender, len(requestIDs))
		}
	}
}
