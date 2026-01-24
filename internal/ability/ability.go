package ability

import "PandaBot/internal/action"

type JobAbility struct {
	English  string
	ID       uint16
	Recast   float32
	LevelReq map[string]int
	Duration float32
	Type     AbilityType
	Targets  action.TargetFlags
	Priority int
}

func (a *JobAbility) GetName() string                    { return a.English }
func (a *JobAbility) GetID() uint16                      { return a.ID }
func (a *JobAbility) GetActionType() action.ActionType   { return action.ActionTypeAbility }
func (a *JobAbility) GetPriority() int                   { return a.Priority }
func (a *JobAbility) GetTargetFlags() action.TargetFlags { return a.Targets }

type AbilityType uint8

const (
	Buff AbilityType = iota
	Cure
	Defensive
	offensive
	Utility
)
