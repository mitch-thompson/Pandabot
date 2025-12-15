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
	castingSystem *casting.CastingServerIntegration
	entityService *entityService.EntityService
}

// NewTriggerService creates a new trigger service
func NewTriggerService(castingSystem *casting.CastingServerIntegration) *TriggerService {
	return &TriggerService{
		castingSystem: castingSystem,
		entityService: entityService.NewEntityService(),
	}
}

// RouteTriggerEvents routes trigger events to the centralized casting system
func (ts *TriggerService) RouteTriggerEvents(triggerEvents []textParser.TriggerEvent, statusMonitor *statusMonitor.StatusMonitor) {
	// Get current party members from status monitor
	partyMembers := statusMonitor.GetAllPartyMembers()
	
	// Convert to entity format for casting system
	entityMembers := ts.entityService.ConvertPartyMembersToEntities(partyMembers)
	
	// Process each trigger event through centralized casting system
	for _, triggerEvent := range triggerEvents {
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