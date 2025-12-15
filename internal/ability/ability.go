package ability

type JobAbility struct {
	English  string
	ID       uint16
	Recast   float32
	LevelReq map[string]int
	Duration float32 // for timed abilities
	Type     AbilityType
}

type AbilityType uint8

const (
	Buff AbilityType = iota
	Cure
	Defensive
	offensive
	Utility
)
