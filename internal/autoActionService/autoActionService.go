package autoActionService

import (
	"fmt"
	"sort"

	"PandaBot/internal/casting"
	"PandaBot/internal/entity"
	"PandaBot/internal/protocol"
	"PandaBot/internal/statusMonitor"
)

// AutoActionService handles automatic actions based on party status
type AutoActionService struct {
	castingSystem *casting.CastingServerIntegration
	PLSource      *string
	PLTarget      *string
	DisableCures  *bool
}

// NewAutoActionService creates a new auto action service
func NewAutoActionService(castingSystem *casting.CastingServerIntegration) *AutoActionService {
	return &AutoActionService{
		castingSystem: castingSystem,
	}
}

// DecideNextAction determines the next action for a client based on the decision tree
func (aas *AutoActionService) DecideNextAction(playerName string, sm *statusMonitor.StatusMonitor) (*protocol.ExecuteCommand, string, error) {
	// 0. Power Leveling Mode logic
	if aas.PLSource != nil && *aas.PLSource == playerName {
		// In PL mode, the source player (being power leveled) should ideally only send data.
		// If DisableCures is explicitly true, we definitely don't cast cures/na.
		// If it's NOT true, we currently stop everything.
		if aas.DisableCures == nil || !*aas.DisableCures {
			return nil, "Power Leveling Mode: Source player is not casting", nil
		}
	}

	for _, statusID := range sm.PlayerStatus {
		if statusID == 6 || statusID == 10 { // Silence IDs
			if sm.EchoDropCount > 0 {
				return &protocol.ExecuteCommand{
					Command: "/item \"Echo Drops\" <me>",
				}, "Silenced - Using Echo Drops", nil
			}
			return nil, "Silenced - No Echo Drops", nil
		}
	}

	partyMap := sm.GetAllPartyMembers()

	// If this is the PL target, consider the PL source's party members
	isPL := false
	if aas.PLTarget != nil && *aas.PLTarget == playerName && aas.PLSource != nil && *aas.PLSource != "" {
		isPL = true
		sourceClient := aas.findClientByName(*aas.PLSource)
		if sourceClient != nil {
			sourceSM := sourceClient.GetStatusMonitor()
			if sourceSM != nil {
				// Add PL source's party members to our partyMap for healing consideration
				sourceParty := sourceSM.GetAllPartyMembers()
				for name, member := range sourceParty {
					if _, exists := partyMap[name]; !exists {
						partyMap[name] = member
					}
				}
			}
		}
	}

	partyEntities := buildPartyEntitiesFromMap(partyMap)

	client := aas.findClientByName(playerName)
	if client == nil {
		return nil, "", fmt.Errorf("client %s not found", playerName)
	}
	clientInfo := client.GetClientInfo()
	if clientInfo == nil {
		return nil, "", fmt.Errorf("client info for %s not found", playerName)
	}

	for _, statusID := range sm.PlayerStatus {
		if statusID == 2 { // Sleep
			return nil, "Caster is slept", nil
		}
	}

	criticalMembers := make([]*statusMonitor.PartyMember, 0)
	if aas.DisableCures == nil || !*aas.DisableCures {
		for _, member := range partyMap {
			if sm.GetHealthThreshold(member.HPPercent) == "critical" {
				criticalMembers = append(criticalMembers, member)
			}
		}
	}

	if len(criticalMembers) > 0 {
		target := criticalMembers[0].Name
		missingHP := criticalMembers[0].HPMax - criticalMembers[0].HPActual
		if criticalMembers[0].HPMax == 0 {
			missingHP = 100 - criticalMembers[0].HPPercent
		}

		var targetEntity *entity.Entity
		for _, e := range partyEntities {
			if e.Name == target {
				targetEntity = e
				break
			}
		}

		cureOption, err := aas.castingSystem.GetCastingEngine().SelectOptimalCure(&casting.CastContext{
			MissingHP:       missingHP,
			CasterMP:        clientInfo.MP,
			CasterJobLevels: clientInfo.JobLevels,
			TargetEntity:    targetEntity,
			PartyMembers:    partyEntities,
			PartySize:       len(partyEntities),
			IsPowerleveling: isPL,
		})

		if err == nil {
			return &protocol.ExecuteCommand{
				Command: fmt.Sprintf("/ma \"%s\" %s", cureOption.SpellName, target),
			}, fmt.Sprintf("Critical Cure: %s on %s", cureOption.SpellName, target), nil
		}
	}

	sleptMembers := make([]string, 0)
	if aas.DisableCures == nil || !*aas.DisableCures {
		for _, member := range partyMap {
			for _, statusID := range member.StatusIDs {
				if statusID == 2 {
					sleptMembers = append(sleptMembers, member.Name)
					break
				}
			}
		}
	}

	if len(sleptMembers) > 1 && !isPL {
		return &protocol.ExecuteCommand{
			Command: fmt.Sprintf("/ma \"Curaga\" %s", sleptMembers[0]),
		}, fmt.Sprintf("Waking up multiple members: %v", sleptMembers), nil
	} else if len(sleptMembers) >= 1 {
		return &protocol.ExecuteCommand{
			Command: fmt.Sprintf("/ma \"Cure\" %s", sleptMembers[0]),
		}, fmt.Sprintf("Waking up member: %s", sleptMembers[0]), nil
	}

	if aas.DisableCures == nil || !*aas.DisableCures {
		for _, member := range partyMap {
			effect := sm.GetMostSevereStatusEffect(member)
			if effect != nil && effect.Severity >= 3 {
				spellName := effect.NaSpell
				if opt, err := aas.castingSystem.GetCastingEngine().SelectOptimalNaAction(&casting.CastContext{
					CasterMP:        clientInfo.MP,
					CasterJobLevels: clientInfo.JobLevels,
					StatusEffects:   member.StatusIDs,
					IsPowerleveling: isPL,
				}); err == nil && opt != nil {
					spellName = opt.GetName()
				}

				return &protocol.ExecuteCommand{
					Command: fmt.Sprintf("/ma \"%s\" %s", spellName, member.Name),
				}, fmt.Sprintf("High priority debuff: %s on %s", effect.Name, member.Name), nil
			}
		}
	}

	// 4. Mid priority cures? (Low threshold)
	lowHPMembers := make([]*statusMonitor.PartyMember, 0)
	if aas.DisableCures == nil || !*aas.DisableCures {
		for _, member := range partyMap {
			if sm.GetHealthThreshold(member.HPPercent) == "low" {
				lowHPMembers = append(lowHPMembers, member)
			}
		}
	}

	if len(lowHPMembers) > 0 {
		target := lowHPMembers[0].Name
		missingHP := lowHPMembers[0].HPMax - lowHPMembers[0].HPActual
		if lowHPMembers[0].HPMax == 0 {
			missingHP = 100 - lowHPMembers[0].HPPercent
		}

		// Find the entity for the target
		var targetEntity *entity.Entity
		for _, e := range partyEntities {
			if e.Name == target {
				targetEntity = e
				break
			}
		}

		cureOption, err := aas.castingSystem.GetCastingEngine().SelectOptimalCure(&casting.CastContext{
			MissingHP:       missingHP,
			CasterMP:        clientInfo.MP,
			CasterJobLevels: clientInfo.JobLevels,
			TargetEntity:    targetEntity,
			PartyMembers:    partyEntities,
			PartySize:       len(partyEntities),
			IsPowerleveling: isPL,
		})

		if err == nil {
			return &protocol.ExecuteCommand{
				Command: fmt.Sprintf("/ma \"%s\" %s", cureOption.SpellName, target),
			}, fmt.Sprintf("Mid priority cure: %s on %s", cureOption.SpellName, target), nil
		}
	}

	// 4.5 Sublimation logic (SCH only)
	isSCH := clientInfo.JobLevels["SCH"] > 0
	if isSCH {
		hasRefresh := false
		hasSubActivated := false
		hasSubComplete := false
		for _, id := range sm.PlayerStatus {
			if id == 43 { // Refresh
				hasRefresh = true
			}
			if id == 187 { // Sublimation: Activated
				hasSubActivated = true
			}
			if id == 188 { // Sublimation: Complete
				hasSubComplete = true
			}
		}

		// - If we don't currently have refresh we should use sublimation (and it's not already active)
		if !hasRefresh && !hasSubActivated && !hasSubComplete {
			return &protocol.ExecuteCommand{
				Command: "/ja \"Sublimation\" <me>",
			}, "Sublimation: No Refresh", nil
		}

		// - When sublimation is full and we are missing mp we should use sublimation
		if hasSubComplete && clientInfo.MP < 100 { // Heuristic: Missing some MP. ClientInfo doesn't have MaxMP currently.
			return &protocol.ExecuteCommand{
				Command: "/ja \"Sublimation\" <me>",
			}, "Sublimation: Releasing full charge", nil
		}

		// - When we are less than 10% mp we should use sublimation if we have it up
		// We don't have MPPercent in ClientInfo, but we can assume MP < 50-100 is low for a SCH
		if clientInfo.MP < 50 && (hasSubActivated || hasSubComplete) {
			return &protocol.ExecuteCommand{
				Command: "/ja \"Sublimation\" <me>",
			}, "Sublimation: Emergency MP recovery", nil
		}
	}

	// 5. Missing desired buffs? (sorted by priority)
	var missingBuffs []struct {
		member *statusMonitor.PartyMember
		id     int
		buff   statusMonitor.DesiredBuff
	}

	for _, member := range partyMap {
		for id, buff := range member.DesiredBuffs {
			// Determine who should be monitored for this buff
			monitoredMember := member

			// Resolve spell to check its targeting
			spellName := buff.SpellName
			if spellName == "reraise" {
				if opt, err := aas.castingSystem.GetCastingEngine().SelectOptimalReraise(clientInfo.JobLevels, clientInfo.MP, aas.castingSystem.GetCastingEngine().Player); err == nil {
					spellName = opt.SpellName
				}
			} else if spellName == "protect" {
				if opt, err := aas.castingSystem.GetCastingEngine().SelectOptimalProtect(&casting.CastContext{
					CasterMP:        clientInfo.MP,
					CasterJobLevels: clientInfo.JobLevels,
					PartySize:       sm.GetPartyCount(),
					IsPowerleveling: isPL,
				}); err == nil {
					spellName = opt.SpellName
				}
			} else if spellName == "shell" {
				if opt, err := aas.castingSystem.GetCastingEngine().SelectOptimalShell(&casting.CastContext{
					CasterMP:        clientInfo.MP,
					CasterJobLevels: clientInfo.JobLevels,
					PartySize:       sm.GetPartyCount(),
					IsPowerleveling: isPL,
				}); err == nil {
					spellName = opt.SpellName
				}
			}

			// Check if this spell is TargetSelf
			if resolvedTarget, err := aas.castingSystem.GetCastingEngine().ResolveActionTarget(spellName, member.Name, &casting.CastContext{
				CasterName:      playerName,
				IsPowerleveling: isPL,
			}); err == nil && (resolvedTarget == playerName || resolvedTarget == "<me>") {
				// For TargetSelf spells, we monitor the caster
				if caster, exists := partyMap[playerName]; exists {
					monitoredMember = caster
				} else {
					// If caster is not in partyMap, we can't monitor them, so skip this buff
					continue
				}
			}

			// SPECIAL CASE: Check if this is a "reraise" buff (ID 113)
			// Status ID 113 is for Reraise, but there's also Reraise II (ID 129) and Reraise III (ID 141)
			// We need to check if the player HAS ANY Reraise status, not just the base ID.
			isReraiseBuff := (id == 113 || id == 129 || id == 141)

			hasBuff := false
			for _, currentID := range monitoredMember.StatusIDs {
				if currentID == id {
					hasBuff = true
					break
				}
				// If we are looking for reraise and have ANY reraise, it counts
				if isReraiseBuff && (currentID == 113 || currentID == 129 || currentID == 141) {
					hasBuff = true
					break
				}
			}
			if !hasBuff {
				missingBuffs = append(missingBuffs, struct {
					member *statusMonitor.PartyMember
					id     int
					buff   statusMonitor.DesiredBuff
				}{member, id, buff})
			}
		}
	}

	if len(missingBuffs) > 0 {
		// Sort by priority descending
		sort.Slice(missingBuffs, func(i, j int) bool {
			return missingBuffs[i].buff.Priority > missingBuffs[j].buff.Priority
		})

		topBuff := missingBuffs[0]
		// Determine initial target
		target := topBuff.member.Name
		spellName := topBuff.buff.SpellName

		// Handle spell resolution for generic names like "reraise", "protect", etc.
		if spellName == "reraise" {
			if opt, err := aas.castingSystem.GetCastingEngine().SelectOptimalReraise(clientInfo.JobLevels, clientInfo.MP, aas.castingSystem.GetCastingEngine().Player); err == nil {
				spellName = opt.SpellName
			}
		} else if spellName == "protect" {
			if opt, err := aas.castingSystem.GetCastingEngine().SelectOptimalProtect(&casting.CastContext{
				CasterMP:        clientInfo.MP,
				CasterJobLevels: clientInfo.JobLevels,
				PartySize:       sm.GetPartyCount(),
				IsPowerleveling: isPL,
			}); err == nil {
				spellName = opt.SpellName
			}
		} else if spellName == "shell" {
			if opt, err := aas.castingSystem.GetCastingEngine().SelectOptimalShell(&casting.CastContext{
				CasterMP:        clientInfo.MP,
				CasterJobLevels: clientInfo.JobLevels,
				PartySize:       sm.GetPartyCount(),
				IsPowerleveling: isPL,
			}); err == nil {
				spellName = opt.SpellName
			}
		}

		// Re-resolve target based on the final spell name (handles TargetSelf, etc.)
		if resolvedTarget, err := aas.castingSystem.GetCastingEngine().ResolveActionTarget(spellName, target, &casting.CastContext{
			CasterName:      "<me>", // autoActionService always assumes caster is <me> for command generation
			IsPowerleveling: isPL,
		}); err == nil {
			target = resolvedTarget
		}

		return &protocol.ExecuteCommand{
			Command: fmt.Sprintf("/ma \"%s\" %s", spellName, target),
		}, fmt.Sprintf("Buffing: %s on %s", spellName, target), nil
	}

	// 6. Low priority debuffs to remove? (Severity 2)
	if aas.DisableCures == nil || !*aas.DisableCures {
		for _, member := range partyMap {
			effect := sm.GetMostSevereStatusEffect(member)
			if effect != nil && effect.Severity == 2 {
				spellName := effect.NaSpell
				if opt, err := aas.castingSystem.GetCastingEngine().SelectOptimalNaAction(&casting.CastContext{
					CasterMP:        clientInfo.MP,
					CasterJobLevels: clientInfo.JobLevels,
					StatusEffects:   member.StatusIDs,
					IsPowerleveling: isPL,
				}); err == nil && opt != nil {
					spellName = opt.GetName()
				}
				return &protocol.ExecuteCommand{
					Command: fmt.Sprintf("/ma \"%s\" %s", spellName, member.Name),
				}, fmt.Sprintf("Low priority debuff: %s on %s", effect.Name, member.Name), nil
			}
		}
	}

	return nil, "Idle", nil
}

func (aas *AutoActionService) findClientByName(name string) casting.ClientInterface {
	clients := aas.castingSystem.GetClientManager().GetConnectedClients()
	for _, client := range clients {
		info := client.GetClientInfo()
		if info != nil && info.PlayerName == name {
			return client
		}
	}
	return nil
}

// ProcessAutomaticActions is now deprecated in favor of DecideNextAction called from the server
func (aas *AutoActionService) ProcessAutomaticActions(statusMonitor *statusMonitor.StatusMonitor) {
	// No-op, functionality moved to DecideNextAction
}

// buildPartyEntitiesFromMap converts a map of party members into entity.Entity list (Main Party only)
func buildPartyEntitiesFromMap(partyMap map[string]*statusMonitor.PartyMember) []*entity.Entity {
	entities := make([]*entity.Entity, 0, len(partyMap))
	for name, pm := range partyMap {
		if !pm.InMainParty {
			continue
		}
		e := &entity.Entity{
			Name:        name,
			HPPercent:   uint8(pm.HPPercent),
			MPPercent:   uint8(pm.MPPercent),
			HPcurrent:   uint32(pm.HPActual),
			HPMax:       uint32(pm.HPMax),
			Zone:        uint16(pm.Zone),
			InMainParty: pm.InMainParty,
		}
		entities = append(entities, e)
	}
	return entities
}

// buildPartyEntities converts the status monitor's party view into entity.Entity list (Main Party only)
func buildPartyEntities(sm *statusMonitor.StatusMonitor) []*entity.Entity {
	return buildPartyEntitiesFromMap(sm.GetAllPartyMembers())
}
