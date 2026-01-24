package entityService

import (
	"PandaBot/internal/entity"
	"PandaBot/internal/statusMonitor"
)

// EntityService handles conversion between different entity representations
type EntityService struct{}

// NewEntityService creates a new entity service
func NewEntityService() *EntityService {
	return &EntityService{}
}

// ConvertPartyMembersToEntities converts status monitor party members to entity format
func (es *EntityService) ConvertPartyMembersToEntities(partyMembers map[string]*statusMonitor.PartyMember) []*entity.Entity {
	var entityMembers []*entity.Entity

	for _, member := range partyMembers {
		entityMember := &entity.Entity{
			Name:      member.Name,
			HPPercent: uint8(member.HPPercent),
			MPPercent: uint8(member.MPPercent),
			Job:       getJobNameFromID(member.Job),
			JobLevel:  uint8(75), // Default level, should be updated from status data
		}

		// Use actual HP/MP values if available (from Ashita v4)
		if member.HPActual > 0 {
			entityMember.HPcurrent = uint32(member.HPActual)

			// Use actual HPMax if available from Ashita v4
			if member.HPMax > 0 {
				entityMember.HPMax = uint32(member.HPMax)
			} else if member.HPPercent > 0 && member.HPPercent <= 100 {
				// Fallback: Calculate HPMax from percentage if actual max not available
				calculatedHPMax := uint32(float64(member.HPActual) * 100.0 / float64(member.HPPercent))
				entityMember.HPMax = calculatedHPMax
			} else {
				// If percentage is invalid (0 or >100), estimate based on job/level
				estimatedHPMax := uint32(1000) // Default fallback
				entityMember.HPMax = estimatedHPMax
			}
		}

		// Convert StatusIDs to Buffs array
		for i, statusID := range member.StatusIDs {
			if i < len(entityMember.Buffs) {
				entityMember.Buffs[i] = uint16(statusID)
			}
		}

		entityMembers = append(entityMembers, entityMember)
	}

	return entityMembers
}

// getJobNameFromID converts a job ID to job name - this should be moved to a job service
func getJobNameFromID(jobID int) string {
	jobNames := map[int]string{
		0: "NONE",
		1: "WAR", 2: "MNK", 3: "WHM", 4: "BLM", 5: "RDM", 6: "THF",
		7: "PLD", 8: "DRK", 9: "BST", 10: "BRD", 11: "RNG", 12: "SAM",
		13: "NIN", 14: "DRG", 15: "SMN", 16: "BLU", 17: "COR", 18: "PUP",
		19: "DNC", 20: "SCH", 21: "GEO", 22: "RUN",
	}

	if name, exists := jobNames[jobID]; exists {
		return name
	}
	return "UNK"
}
