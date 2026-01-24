package spell

import "PandaBot/internal/action"

type Spell struct {
	English    string
	ID         uint16 // Packet spell ID
	MPCost     uint16
	CastTime   float32
	Recast     float32
	LevelReq   map[string]int // e.g. {"WHM": 17, "RDM": 21}
	Type       SpellType
	Element    Element
	Targets    action.TargetFlags
	Priority   int // for auto-selection (higher = more important)
	IsAoE      bool
	HealAmount int // For healing spells
}

func (s *Spell) GetName() string                    { return s.English }
func (s *Spell) GetID() uint16                      { return s.ID }
func (s *Spell) GetActionType() action.ActionType   { return action.ActionTypeSpell }
func (s *Spell) GetPriority() int                   { return s.Priority }
func (s *Spell) GetTargetFlags() action.TargetFlags { return s.Targets }

type SpellType uint8

const (
	Healing SpellType = iota
	Enhancing
	Enfeebling
	DarkMagic
	Summoning
	Ninjutsu
	Singing
	BlueMagic
)

type Element uint8

const (
	Fire Element = iota
	Ice
	Wind
	Earth
	Thunder
	Water
	Light
	Dark
)
