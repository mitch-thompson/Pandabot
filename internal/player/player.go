package player

import (
	"PandaBot/internal/ability"
	"PandaBot/internal/entity"
	"PandaBot/internal/spell"
	"time"
)

type Player struct {
	Me       *entity.Entity
	Party    [6]*entity.Entity //p0-p5
	Alliance [18]*entity.Entity

	AvailableSpells map[string]*spell.Spell // name -> spell(fast lookup)
	AvailableJA     map[string]*ability.JobAbility
	SpellRecast     map[uint16]time.Time // spell ID → ready after
	AbilityRecast   map[uint16]time.Time
}

func (p *Player) CanCast(spellName string) bool {
	s, ok := p.AvailableSpells[spellName]
	if !ok {
		return false
	}
	ready, exists := p.SpellRecast[s.ID]
	return !exists || time.Now().After(ready)
}
