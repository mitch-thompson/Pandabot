package action

import (
	"PandaBot/internal/ability"
	"PandaBot/internal/entity"
	"PandaBot/internal/spell"
	"time"
)

type ActionType uint8

const (
	CastSpell ActionType = iota
	UseAbility
	UseItem
	Wait
	Follow
)

type Action struct {
	Type    ActionType
	Spell   *spell.Spell
	Ability *ability.JobAbility
	Target  *entity.Entity
	Delay   time.Duration
}
