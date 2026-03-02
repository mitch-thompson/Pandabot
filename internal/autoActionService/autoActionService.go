package autoActionService

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"PandaBot/internal/casting"
	"PandaBot/internal/cureSelector"
	"PandaBot/internal/entity"
	"PandaBot/internal/player"
	"PandaBot/internal/protocol"
	"PandaBot/internal/statusMonitor"
	"PandaBot/internal/zone"
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

// DecideNextAction determines the next action for a client based on the decision tree
func (aas *AutoActionService) DecideNextAction(playerName string, sm *statusMonitor.StatusMonitor, disableCures bool, plSource string, plTarget string) (*protocol.ExecuteCommand, string, error) {
	// 0. Silence Check - MUST be first to prevent any casting while silenced
	isSilenced := false
	for _, statusID := range sm.PlayerStatus {
		if statusID == 6 || statusID == 10 { // Silence IDs
			isSilenced = true
			break
		}
	}

	// Double check party members for our own name in case PlayerStatus is out of sync
	if !isSilenced {
		if me, exists := sm.GetPartyMember(playerName); exists {
			for _, statusID := range me.StatusIDs {
				if statusID == 6 || statusID == 10 {
					isSilenced = true
					break
				}
			}
		}
	}

	if isSilenced {
		if sm.EchoDropCount > 0 {
			return &protocol.ExecuteCommand{
				Command: "/item \"Echo Drops\" <me>",
			}, "Silenced - Using Echo Drops", nil
		}
		return nil, "Silenced - No Echo Drops", nil
	}

	// 0.1. Power Leveling Mode logic

	partyMap := sm.GetAllPartyMembers()

	// If this is the PL source, return no action (unless cures are disabled)
	if plSource != "" && plTarget != "" && strings.EqualFold(plSource, playerName) {
		if !disableCures {
			return nil, "PL Source Mode: Automatic actions paused", nil
		}
	}

	// If this is the PL target, consider the PL source's party members
	isPL := false
	if plTarget != "" && plSource != "" && strings.EqualFold(plTarget, playerName) {
		isPL = true
		sourceClient := aas.findClientByName(plSource)
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

	p := aas.castingSystem.GetCastingEngine().Player

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

	// Red Mage Convert Logic
	// If RDM > 40, MP < 10%, and Convert is ready, add to queue
	if rdmLevel, ok := clientInfo.JobLevels["RDM"]; ok && rdmLevel >= 40 {
		mpPercent := 100
		if me, exists := sm.GetPartyMember(playerName); exists {
			if me.MPMax > 0 {
				mpPercent = (me.MPActual * 100) / me.MPMax
			} else {
				mpPercent = me.MPPercent
			}
		}

		if mpPercent < 10 && client.CanUseAbility("Convert") {
			return &protocol.ExecuteCommand{
				Command:  "/ja \"Convert\" <me>",
				Priority: 80, // High priority
			}, "MP low (< 10%) - Using Convert", nil
		}
	}

	criticalMembers := make([]*statusMonitor.PartyMember, 0)
	if !disableCures {
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
			Player:          p,
			MissingHP:       missingHP,
			CasterMP:        clientInfo.MP,
			CasterJobLevels: clientInfo.JobLevels,
			TargetEntity:    targetEntity,
			PartyMembers:    partyEntities,
			PartySize:       len(partyEntities),
			IsPowerleveling: isPL,
		})

		if err == nil {
			// SCH Accession: replace Curaga with Accession + single-target Cure
			if strings.HasPrefix(cureOption.SpellName, "Curaga") {
				if cmd, reason := aas.handleAccessionCure(sm, client, playerName, cureOption, target, p, clientInfo, isPL); cmd != nil {
					return cmd, "Critical " + reason, nil
				}
			}

			// Resolve target based on spell targeting flags
			spellTarget := target
			if resolved, err := aas.castingSystem.GetCastingEngine().ResolveActionTarget(cureOption.SpellName, target, &casting.CastContext{
				Player:          p,
				CasterName:      playerName,
				IsPowerleveling: isPL,
			}); err == nil {
				spellTarget = resolved
			}

			return &protocol.ExecuteCommand{
				Command: fmt.Sprintf("/ma \"%s\" %s", cureOption.SpellName, spellTarget),
			}, fmt.Sprintf("Critical Cure: %s on %s", cureOption.SpellName, target), nil
		}
	}

	sleptMembers := make([]string, 0)
	if !disableCures {
		for _, member := range partyMap {
			for _, statusID := range member.StatusIDs {
				if statusID == 2 {
					sleptMembers = append(sleptMembers, member.Name)
					break
				}
			}
		}
	}

	if len(sleptMembers) > 1 {
		if isPL {
			return &protocol.ExecuteCommand{
				Command: fmt.Sprintf("/ma \"Cure\" %s", sleptMembers[0]),
			}, fmt.Sprintf("Waking up multiple members (Curaga disabled in PL): %v", sleptMembers), nil
		}
		return &protocol.ExecuteCommand{
			Command: fmt.Sprintf("/ma \"Curaga\" %s", sleptMembers[0]),
		}, fmt.Sprintf("Waking up multiple members: %v", sleptMembers), nil
	} else if len(sleptMembers) >= 1 {
		return &protocol.ExecuteCommand{
			Command: fmt.Sprintf("/ma \"Cure\" %s", sleptMembers[0]),
		}, fmt.Sprintf("Waking up member: %s", sleptMembers[0]), nil
	}

	if !disableCures {
		for _, member := range partyMap {
			effect := sm.GetMostSevereStatusEffect(member)
			if effect != nil && effect.Severity >= 3 {
				if isPL {
					continue
				}
				spellName := effect.NaSpell
				if opt, err := aas.castingSystem.GetCastingEngine().SelectOptimalNaAction(&casting.CastContext{
					Player:          p,
					CasterMP:        clientInfo.MP,
					CasterJobLevels: clientInfo.JobLevels,
					StatusEffects:   member.StatusIDs,
					IsPowerleveling: isPL,
				}); err == nil && opt != nil {
					spellName = opt.GetName()
				}

				// Resolve target based on spell targeting flags
				spellTarget := member.Name
				if resolved, err := aas.castingSystem.GetCastingEngine().ResolveActionTarget(spellName, member.Name, &casting.CastContext{
					Player:          p,
					CasterName:      playerName,
					IsPowerleveling: isPL,
				}); err == nil {
					spellTarget = resolved
				}

				return &protocol.ExecuteCommand{
					Command: fmt.Sprintf("/ma \"%s\" %s", spellName, spellTarget),
				}, fmt.Sprintf("High priority debuff: %s on %s", effect.Name, member.Name), nil
			}
		}
	}

	// 4. Mid priority cures? (Low threshold)
	lowHPMembers := make([]*statusMonitor.PartyMember, 0)
	if !disableCures {
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
			Player:          p,
			MissingHP:       missingHP,
			CasterMP:        clientInfo.MP,
			CasterJobLevels: clientInfo.JobLevels,
			TargetEntity:    targetEntity,
			PartyMembers:    partyEntities,
			PartySize:       len(partyEntities),
			IsPowerleveling: isPL,
		})

		if err == nil {
			// SCH Accession: replace Curaga with Accession + single-target Cure
			if strings.HasPrefix(cureOption.SpellName, "Curaga") {
				if cmd, reason := aas.handleAccessionCure(sm, client, playerName, cureOption, target, p, clientInfo, isPL); cmd != nil {
					return cmd, reason, nil
				}
			}

			// Resolve target based on spell targeting flags
			spellTarget := target
			if resolved, err := aas.castingSystem.GetCastingEngine().ResolveActionTarget(cureOption.SpellName, target, &casting.CastContext{
				Player:          p,
				CasterName:      playerName,
				IsPowerleveling: isPL,
			}); err == nil {
				spellTarget = resolved
			}

			return &protocol.ExecuteCommand{
				Command: fmt.Sprintf("/ma \"%s\" %s", cureOption.SpellName, spellTarget),
			}, fmt.Sprintf("Mid priority cure: %s on %s", cureOption.SpellName, target), nil
		}
	}

	// 4.5. Auto-register Light Arts and Addendum: White for SCH players not in town
	if schLevel, ok := clientInfo.JobLevels["SCH"]; ok && schLevel >= 10 {
		if me, exists := sm.GetPartyMember(playerName); exists {
			zoneStr := fmt.Sprintf("Zone_%d", me.Zone)
			if !zone.IsRestricted(zoneStr) {
				// Check if Light Arts or Addendum: White is active
				hasLightArts := false
				hasAddendumWhite := false
				for _, sid := range me.StatusIDs {
					if sid == 358 {
						hasLightArts = true
					}
					if sid == 401 {
						hasAddendumWhite = true
					}
				}
				// Register Light Arts as a desired ability buff if neither Light Arts nor Addendum: White is active
				if !hasLightArts && !hasAddendumWhite {
					if _, hasBuff := me.DesiredBuffs[358]; !hasBuff {
						sm.RegisterDesiredAbilityBuff(playerName, 358, "Light Arts", 80, time.Time{})
					}
				}
				// Register Addendum: White if SCH >= 30 and Light Arts (or Addendum: White) is active
				if schLevel >= 30 {
					if hasLightArts || hasAddendumWhite {
						if _, hasBuff := me.DesiredBuffs[401]; !hasBuff {
							sm.RegisterDesiredAbilityBuff(playerName, 401, "Addendum: White", 79, time.Time{})
						}
					}
				}
			}
		}
	}

	// 4.6. Sublimation handling for SCH players
	if schLevel, ok := clientInfo.JobLevels["SCH"]; ok && schLevel >= 10 {
		if me, exists := sm.GetPartyMember(playerName); exists {
			zoneStr := fmt.Sprintf("Zone_%d", me.Zone)
			if !zone.IsRestricted(zoneStr) {
				hasSublimationActivated := false
				hasSublimationComplete := false
				for _, sid := range me.StatusIDs {
					if sid == 187 {
						hasSublimationActivated = true
					}
					if sid == 188 {
						hasSublimationComplete = true
					}
				}

				mpPercent := 100
				if me.MPMax > 0 {
					mpPercent = (me.MPActual * 100) / me.MPMax
				} else {
					mpPercent = me.MPPercent
				}

				// Use Sublimation to recover MP if complete and <50% MP, or activated and <20% MP
				if hasSublimationComplete && mpPercent < 50 {
					if client.CanUseAbility("Sublimation") {
						return &protocol.ExecuteCommand{
							Command:  "/ja \"Sublimation\" <me>",
							Priority: 85,
						}, "Sublimation Complete - Recovering MP (< 50%)", nil
					}
					return nil, "Sublimation Complete but ability not ready", nil
				}
				if hasSublimationActivated && mpPercent < 20 {
					if client.CanUseAbility("Sublimation") {
						return &protocol.ExecuteCommand{
							Command:  "/ja \"Sublimation\" <me>",
							Priority: 85,
						}, "Sublimation Activated - Recovering MP (< 20%)", nil
					}
					return nil, "Sublimation Activated but ability not ready", nil
				}

				// Activate Sublimation if neither status is present
				if !hasSublimationActivated && !hasSublimationComplete {
					if client.CanUseAbility("Sublimation") {
						return &protocol.ExecuteCommand{
							Command:  "/ja \"Sublimation\" <me>",
							Priority: 30,
						}, "Activating Sublimation", nil
					}
				}
			}
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
					Player:          p,
					CasterMP:        clientInfo.MP,
					CasterJobLevels: clientInfo.JobLevels,
					PartySize:       sm.GetPartyCount(),
					IsPowerleveling: isPL,
				}); err == nil {
					spellName = opt.SpellName
				}
			} else if spellName == "shell" {
				if opt, err := aas.castingSystem.GetCastingEngine().SelectOptimalShell(&casting.CastContext{
					Player:          p,
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
				Player:          p,
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

			// SPECIAL CASE: Light Arts (358) is implicitly active when Addendum: White (401) is up
			isLightArtsBuff := (id == 358)

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
				// If we are looking for Light Arts and have Addendum: White, Light Arts is implicitly active
				if isLightArtsBuff && currentID == 401 {
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
				Player:          p,
				CasterMP:        clientInfo.MP,
				CasterJobLevels: clientInfo.JobLevels,
				PartySize:       sm.GetPartyCount(),
				IsPowerleveling: isPL,
			}); err == nil {
				spellName = opt.SpellName
			}
		} else if spellName == "shell" {
			if opt, err := aas.castingSystem.GetCastingEngine().SelectOptimalShell(&casting.CastContext{
				Player:          p,
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
			Player:          p,
			CasterName:      "<me>", // autoActionService always assumes caster is <me> for command generation
			IsPowerleveling: isPL,
		}); err == nil {
			target = resolvedTarget
		}

		// Use /ja for ability buffs, /ma for spell buffs
		if topBuff.buff.IsAbility {
			if client.CanUseAbility(spellName) {
				return &protocol.ExecuteCommand{
					Command:  fmt.Sprintf("/ja \"%s\" <me>", spellName),
					Priority: topBuff.buff.Priority,
				}, fmt.Sprintf("Using ability: %s", spellName), nil
			}
			// Ability not ready, skip to next action
			return nil, fmt.Sprintf("Ability %s not ready", spellName), nil
		}

		return &protocol.ExecuteCommand{
			Command: fmt.Sprintf("/ma \"%s\" %s", spellName, target),
		}, fmt.Sprintf("Buffing: %s on %s", spellName, target), nil
	}

	// 6. Low priority debuffs to remove? (Severity 2)
	if !disableCures {
		for _, member := range partyMap {
			effect := sm.GetMostSevereStatusEffect(member)
			if effect != nil && effect.Severity == 2 {
				if isPL {
					continue
				}
				spellName := effect.NaSpell
				if opt, err := aas.castingSystem.GetCastingEngine().SelectOptimalNaAction(&casting.CastContext{
					Player:          p,
					CasterMP:        clientInfo.MP,
					CasterJobLevels: clientInfo.JobLevels,
					StatusEffects:   member.StatusIDs,
					IsPowerleveling: isPL,
				}); err == nil && opt != nil {
					spellName = opt.GetName()
				}

				// Resolve target based on spell targeting flags
				spellTarget := member.Name
				if resolved, err := aas.castingSystem.GetCastingEngine().ResolveActionTarget(spellName, member.Name, &casting.CastContext{
					Player:          p,
					CasterName:      playerName,
					IsPowerleveling: isPL,
				}); err == nil {
					spellTarget = resolved
				}

				return &protocol.ExecuteCommand{
					Command: fmt.Sprintf("/ma \"%s\" %s", spellName, spellTarget),
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
		if info != nil && strings.EqualFold(info.PlayerName, name) {
			return client
		}
	}
	return nil
}

// ProcessAutomaticActions is now deprecated in favor of DecideNextAction called from the server
func (aas *AutoActionService) ProcessAutomaticActions(statusMonitor *statusMonitor.StatusMonitor) {
	// No-op, functionality moved to DecideNextAction
}

// curagaToSingleCure maps Curaga spell names to their single-target Cure equivalents.
var curagaToSingleCure = map[string]string{
	"Curaga":     "Cure II",
	"Curaga II":  "Cure III",
	"Curaga III": "Cure IV",
	"Curaga IV":  "Cure V",
	"Curaga V":   "Cure VI",
}

// handleAccessionCure checks if SCH can use Accession to turn a single-target cure into AoE,
// replacing curaga. Returns a command and reason if handled, or nil if curaga should proceed normally.
func (aas *AutoActionService) handleAccessionCure(
	sm *statusMonitor.StatusMonitor,
	client casting.ClientInterface,
	playerName string,
	cureOption *cureSelector.CureOption,
	target string,
	p *player.Player,
	clientInfo *casting.ClientInfo,
	isPL bool,
) (*protocol.ExecuteCommand, string) {
	if _, ok := clientInfo.JobLevels["SCH"]; !ok {
		return nil, ""
	}
	if sm.GetStratagemCount() < 2 {
		return nil, ""
	}

	me, exists := sm.GetPartyMember(playerName)
	if !exists {
		return nil, ""
	}

	hasAccession := false
	for _, sid := range me.StatusIDs {
		if sid == 399 {
			hasAccession = true
			break
		}
	}

	if !hasAccession {
		if client.CanUseAbility("Accession") {
			return &protocol.ExecuteCommand{
				Command:  "/ja \"Accession\" <me>",
				Priority: 90,
			}, "SCH Accession (preparing AoE cure)"
		}
		return nil, ""
	}

	cureName, ok := curagaToSingleCure[cureOption.SpellName]
	if !ok {
		cureName = "Cure"
	}

	spellTarget := target
	if resolved, err := aas.castingSystem.GetCastingEngine().ResolveActionTarget(cureName, target, &casting.CastContext{
		Player:          p,
		CasterName:      playerName,
		IsPowerleveling: isPL,
	}); err == nil {
		spellTarget = resolved
	}

	return &protocol.ExecuteCommand{
		Command: fmt.Sprintf("/ma \"%s\" %s", cureName, spellTarget),
	}, fmt.Sprintf("SCH Accession Cure: %s on %s (AoE via Accession)", cureName, spellTarget)
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
