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
	// If we don't have any known-spell data yet, do not block selection
	if len(p.AvailableSpells) == 0 {
		return true
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
	return time.Now().After(ready)
}

func (p *Player) SetSpellRecast(spellID uint16, readyAt time.Time) {
	if p.SpellRecast == nil {
		p.SpellRecast = make(map[uint16]time.Time)
	}
	p.SpellRecast[spellID] = readyAt
	// log.Printf("[DEBUG] SetSpellRecast ID:%d -> %v", spellID, readyAt)
}

func (p *Player) CanUseAbility(abilityName string) bool {
	if len(p.AvailableJA) == 0 {
		return true
	}
	a, ok := p.AvailableJA[abilityName]
	if !ok {
		return false
	}
	if p.AbilityRecast == nil {
		return true
	}
	ready, exists := p.AbilityRecast[a.ID]
	if !exists {
		return true
	}
	return time.Now().After(ready)
}

func (p *Player) SetAbilityRecast(abilityID uint16, readyAt time.Time) {
	if p.AbilityRecast == nil {
		p.AbilityRecast = make(map[uint16]time.Time)
	}
	p.AbilityRecast[abilityID] = readyAt
}
