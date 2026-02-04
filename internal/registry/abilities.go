package registry

import (
	"PandaBot/internal/ability"
	"PandaBot/internal/action"
	"fmt"
	"sync"
)

var (
	abilities     = make(map[string]*ability.JobAbility)
	abilitiesByID = make(map[uint16]*ability.JobAbility)
	abilityMu     sync.RWMutex
)

func init() {
	initializeAbilities()
}

func RegisterAbility(a *ability.JobAbility) {
	abilityMu.Lock()
	defer abilityMu.Unlock()
	abilities[a.English] = a
	abilitiesByID[a.ID] = a
}

func GetAbility(name string) (*ability.JobAbility, error) {
	abilityMu.RLock()
	defer abilityMu.RUnlock()
	a, ok := abilities[name]
	if !ok {
		return nil, fmt.Errorf("ability not found: %s", name)
	}
	return a, nil
}

func GetAbilityByID(id uint16) (*ability.JobAbility, error) {
	abilityMu.RLock()
	defer abilityMu.RUnlock()
	a, ok := abilitiesByID[id]
	if !ok {
		return nil, fmt.Errorf("ability ID not found: %d", id)
	}
	return a, nil
}

func initializeAbilities() {
	defaultAbilities := []*ability.JobAbility{
		{English: "Benediction", ID: 1, Recast: 3600, Type: ability.Cure, Targets: action.TargetSelf | action.TargetAoE, Priority: 10, LevelReq: map[string]int{"WHM": 1}},
		{English: "Divine Seal", ID: 2, Recast: 600, Type: ability.Buff, Targets: action.TargetSelf, Priority: 5, LevelReq: map[string]int{"WHM": 15}},
		{English: "Light Arts", ID: 3, Recast: 60, Type: ability.Buff, Targets: action.TargetSelf, Priority: 5, LevelReq: map[string]int{"SCH": 10}},
		{English: "Afflatus Solace", ID: 4, Recast: 60, Type: ability.Buff, Targets: action.TargetSelf, Priority: 5, LevelReq: map[string]int{"WHM": 40}},
		{English: "Devotion", ID: 101, Recast: 600, Type: ability.Utility, Targets: action.TargetPartyMember, Priority: 5, LevelReq: map[string]int{"WHM": 75}},
	}

	for _, a := range defaultAbilities {
		RegisterAbility(a)
	}
}
