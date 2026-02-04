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
	if p.AvailableSpells == nil {
		return false
	}
	s, ok := p.AvailableSpells[spellName]
	if !ok {
		return false
	}
	if p.SpellRecast == nil {
		return true
	}
	ready, exists := p.SpellRecast[s.ID]
	if !exists {
		return true
	}
	isReady := time.Now().After(ready)
	if !isReady {
		// log.Printf("[DEBUG] CanCast(%s) ID:%d - NOT READY until %v (now: %v)", spellName, s.ID, ready, time.Now())
	}
	return isReady
}

func (p *Player) SetSpellRecast(spellID uint16, readyAt time.Time) {
	if p.SpellRecast == nil {
		p.SpellRecast = make(map[uint16]time.Time)
	}
	p.SpellRecast[spellID] = readyAt
	// log.Printf("[DEBUG] SetSpellRecast ID:%d -> %v", spellID, readyAt)
}
