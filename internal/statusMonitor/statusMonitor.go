package statusMonitor

import (
	"PandaBot/internal/status"
	"fmt"
	"strings"
	"time"
)

// StatusMonitor tracks party member states and triggers actions
type StatusMonitor struct {
	partyMembers     map[string]*PartyMember
	healthThresholds HealthThresholds
	statusEffects    map[int]status.StatusInfo
	lastUpdate       time.Time
	updateInterval   time.Duration
	PlayerName       string
	PlayerStatus     []int
	EchoDropCount    int
}

// PartyMember represents a party member's current state
type PartyMember struct {
	Name               string
	HPPercent          int
	MPPercent          int
	HPActual           int // Actual HP value from Ashita v4
	MPActual           int // Actual MP value from Ashita v4
	HPMax              int // Max HP value from Ashita v4
	MPMax              int // Max MP value from Ashita v4
	Job                int
	Zone               int
	StatusIDs          []int
	DesiredBuffs       map[int]DesiredBuff // map of status ID to buff info
	LastSeen           time.Time
	NeedsHealing       bool
	NeedsStatusRemoval bool
	Priority           int  // Higher number = higher priority
	InMainParty        bool // True if in p0-p5, false if in alliance p6-p17
}

// DesiredBuff represents a buff that should be maintained
type DesiredBuff struct {
	SpellName string
	Priority  int
	Expiry    time.Time
}

// HealthThresholds defines when healing is needed
type HealthThresholds struct {
	Critical int // Below this triggers emergency healing
	Low      int // Below this triggers regular healing
	Medium   int // Below this triggers preventive healing
}

// StatusEffectInfo contains information about status effects
// ActionTrigger represents a triggered action
type ActionTrigger struct {
	Type     string // "cure", "na_spell", "buff", "echo_drop", "manual_spell"
	Target   string
	Spell    string
	Priority int
	Reason   string
}

// NewStatusMonitor creates a new status monitor
func NewStatusMonitor() *StatusMonitor {
	return &StatusMonitor{
		partyMembers: make(map[string]*PartyMember),
		healthThresholds: HealthThresholds{
			Critical: 25,
			Low:      50,
			Medium:   75,
		},
		statusEffects:  status.Registry,
		updateInterval: 5 * time.Second,
		PlayerStatus:   make([]int, 0),
	}
}

// UpdatePartyMember updates a party member's status
func (sm *StatusMonitor) UpdatePartyMember(name string, hpPercent, mpPercent, job, zone int, statusIDs []int, inMainParty bool) {
	sm.UpdatePartyMemberWithActuals(name, hpPercent, mpPercent, 0, 0, job, zone, statusIDs, inMainParty)
}

// UpdatePartyMemberWithActuals updates a party member's status including actual HP/MP values
func (sm *StatusMonitor) UpdatePartyMemberWithActuals(name string, hpPercent, mpPercent, hpActual, mpActual, job, zone int, statusIDs []int, inMainParty bool) {
	sm.UpdatePartyMemberWithMaxValues(name, hpPercent, mpPercent, hpActual, mpActual, 0, 0, job, zone, statusIDs, inMainParty)
}

// UpdatePartyMemberWithMaxValues updates a party member's status including actual and max HP/MP values
func (sm *StatusMonitor) UpdatePartyMemberWithMaxValues(name string, hpPercent, mpPercent, hpActual, mpActual, hpMax, mpMax, job, zone int, statusIDs []int, inMainParty bool) {
	member, exists := sm.partyMembers[name]
	if !exists {
		member = &PartyMember{
			Name:         name,
			Priority:     sm.calculateMemberPriority(job),
			DesiredBuffs: make(map[int]DesiredBuff),
			InMainParty:  inMainParty,
		}
		sm.partyMembers[name] = member
	}

	member.HPPercent = hpPercent
	member.MPPercent = mpPercent
	member.HPActual = hpActual
	member.MPActual = mpActual
	member.HPMax = hpMax
	member.MPMax = mpMax
	member.Job = job
	member.Zone = zone
	member.StatusIDs = statusIDs
	member.LastSeen = time.Now()
	member.InMainParty = inMainParty

	// Update needs flags
	member.NeedsHealing = sm.needsHealing(member)
	member.NeedsStatusRemoval = sm.needsStatusRemoval(member)

	sm.lastUpdate = time.Now()
}

// calculateMemberPriority determines priority based on job
func (sm *StatusMonitor) UpdatePlayerStatus(playerName string, statusIDs []int, echoDropCount int) {
	sm.PlayerName = playerName
	sm.PlayerStatus = statusIDs
	sm.EchoDropCount = echoDropCount
}

func (sm *StatusMonitor) calculateMemberPriority(job int) int {
	// Job priority mapping (higher = more important)
	jobPriorities := map[int]int{
		1:  8, // WAR (Warrior)
		2:  6, // MNK (Monk)
		3:  9, // WHM (White Mage) - Highest priority
		4:  8, // BLM (Black Mage)
		5:  7, // RDM (Red Mage)
		6:  5, // THF (Thief)
		7:  8, // PLD (Paladin)
		8:  7, // DRK (Dark Knight)
		9:  6, // BST (Beastmaster)
		10: 7, // BRD (Bard)
		11: 6, // RNG (Ranger)
		12: 6, // SAM (Samurai)
		13: 6, // NIN (Ninja)
		14: 6, // DRG (Dragoon)
		15: 8, // SMN (Summoner)
		16: 6, // BLU (Blue Mage)
		17: 6, // COR (Corsair)
		18: 6, // PUP (Puppetmaster)
		19: 6, // DNC (Dancer)
		20: 7, // SCH (Scholar)
		21: 6, // GEO (Geomancer)
		22: 6, // RUN (Runic Knight)
	}

	if priority, exists := jobPriorities[job]; exists {
		return priority
	}
	return 5 // Default priority
}

// needsHealing determines if a member needs healing
func (sm *StatusMonitor) needsHealing(member *PartyMember) bool {
	return member.HPPercent < sm.healthThresholds.Medium
}

// needsStatusRemoval determines if a member needs status removal
func (sm *StatusMonitor) needsStatusRemoval(member *PartyMember) bool {
	for _, statusID := range member.StatusIDs {
		if effect, exists := sm.statusEffects[statusID]; exists {
			if effect.Severity >= 2 { // Moderate or higher severity
				return true
			}
		}
	}
	return false
}

// GetHealthThreshold returns the appropriate health threshold category
func (sm *StatusMonitor) GetHealthThreshold(hpPercent int) string {
	if hpPercent <= sm.healthThresholds.Critical {
		return "critical"
	} else if hpPercent <= sm.healthThresholds.Low {
		return "low"
	} else if hpPercent <= sm.healthThresholds.Medium {
		return "medium"
	}
	return "healthy"
}

// GetMostSevereStatusEffect returns the most severe status effect for a member
func (sm *StatusMonitor) GetMostSevereStatusEffect(member *PartyMember) *status.StatusInfo {
	var mostSevere *status.StatusInfo
	maxSeverity := 0

	for _, statusID := range member.StatusIDs {
		if effect, exists := sm.statusEffects[statusID]; exists {
			if effect.Severity > maxSeverity {
				maxSeverity = effect.Severity
				// Create a local copy to avoid referencing the loop variable 'effect'
				temp := effect
				mostSevere = &temp
			}
		}
	}

	return mostSevere
}

// CheckForActions analyzes current party state and returns triggered actions
func (sm *StatusMonitor) CheckForActions() []ActionTrigger {
	var actions []ActionTrigger

	// Check for silence (echo drop) first - highest priority (Requirement 10.1)
	for _, statusID := range sm.PlayerStatus {
		if statusID == 6 { // Silence
			if sm.EchoDropCount > 0 {
				actions = append(actions, ActionTrigger{
					Type:     "echo_drop",
					Target:   "<me>",
					Spell:    "Echo Drop",
					Priority: 10,
					Reason:   "Silenced",
				})
			}
			break
		}
	}

	for _, member := range sm.partyMembers {
		// Check for healing needs
		if member.NeedsHealing {
			threshold := sm.GetHealthThreshold(member.HPPercent)
			priority := sm.calculateHealingPriority(member, threshold)

			action := ActionTrigger{
				Type:     "cure",
				Target:   member.Name,
				Priority: priority,
				Reason:   fmt.Sprintf("HP at %d%% (%s)", member.HPPercent, threshold),
			}
			actions = append(actions, action)
		}

		// Check for status removal needs
		if member.NeedsStatusRemoval {
			effect := sm.GetMostSevereStatusEffect(member)
			if effect != nil {
				priority := sm.calculateStatusRemovalPriority(member, effect)

				action := ActionTrigger{
					Type:     "na_spell",
					Target:   member.Name,
					Spell:    effect.NaSpell,
					Priority: priority,
					Reason:   fmt.Sprintf("Has %s (severity %d)", effect.Name, effect.Severity),
				}
				actions = append(actions, action)
			}
		}

		// Check for desired buffs
		for statusID, buff := range member.DesiredBuffs {
			hasBuff := false
			for _, currentID := range member.StatusIDs {
				if currentID == statusID {
					hasBuff = true
					break
				}
			}

			if !hasBuff {
				// Check for expiry
				if !buff.Expiry.IsZero() && time.Now().After(buff.Expiry) {
					delete(member.DesiredBuffs, statusID)
					continue
				}

				// TODO: check zone logic (for now just recast)
				actions = append(actions, ActionTrigger{
					Type:     "manual_spell", // Or "buff" if we want optimal selection
					Target:   member.Name,
					Spell:    buff.SpellName,
					Priority: buff.Priority,
					Reason:   fmt.Sprintf("Desired buff %s (ID %d) is missing", buff.SpellName, statusID),
				})
			}
		}
	}

	return actions
}

// calculateHealingPriority determines healing priority
func (sm *StatusMonitor) calculateHealingPriority(member *PartyMember, threshold string) int {
	basePriority := member.Priority

	switch threshold {
	case "critical":
		return basePriority + 100 // Highest priority
	case "low":
		return basePriority + 50
	case "medium":
		return basePriority + 20
	default:
		return basePriority
	}
}

// calculateStatusRemovalPriority determines status removal priority
func (sm *StatusMonitor) calculateStatusRemovalPriority(member *PartyMember, effect *status.StatusInfo) int {
	basePriority := member.Priority
	severityBonus := effect.Severity * 15

	return basePriority + severityBonus
}

// GetPartyMember returns a party member by name
func (sm *StatusMonitor) GetPartyMember(name string) (*PartyMember, bool) {
	member, exists := sm.partyMembers[name]
	return member, exists
}

// GetAllPartyMembers returns all party members
func (sm *StatusMonitor) GetAllPartyMembers() map[string]*PartyMember {
	return sm.partyMembers
}

// RemovePartyMember removes a party member (when they leave)
func (sm *StatusMonitor) RemovePartyMember(name string) {
	delete(sm.partyMembers, name)
}

// CleanupStaleMembers removes members not seen recently
func (sm *StatusMonitor) CleanupStaleMembers(maxAge time.Duration) int {
	removed := 0
	cutoff := time.Now().Add(-maxAge)

	for name, member := range sm.partyMembers {
		if member.LastSeen.Before(cutoff) {
			delete(sm.partyMembers, name)
			removed++
		}
	}

	return removed
}

// RegisterDesiredBuff adds a buff that should be monitored for a player
func (sm *StatusMonitor) RegisterDesiredBuff(playerName string, statusID int, spellName string, priority int, expiry time.Time) {
	targetName := playerName
	if targetName == "<me>" {
		targetName = sm.PlayerName
	}

	if targetName == "" {
		return
	}

	member, exists := sm.partyMembers[targetName]
	if !exists {
		return
	}
	member.DesiredBuffs[statusID] = DesiredBuff{
		SpellName: spellName,
		Priority:  priority,
		Expiry:    expiry,
	}
}

// ClearDesiredBuffs clears all registered desired buffs for all party members
func (sm *StatusMonitor) ClearDesiredBuffs() {
	for _, member := range sm.partyMembers {
		member.DesiredBuffs = make(map[int]DesiredBuff)
	}
}

// ClearDesiredBuff clears buffs for a specific target
func (sm *StatusMonitor) ClearDesiredBuff(target string) {
	if target == "<me>" {
		target = sm.PlayerName
	}
	if member, exists := sm.partyMembers[target]; exists {
		member.DesiredBuffs = make(map[int]DesiredBuff)
	}
}

// ClearDesiredBuffBySpell clears a specific buff type across all members
func (sm *StatusMonitor) ClearDesiredBuffBySpell(spellName string) {
	spellName = strings.ToLower(spellName)
	for _, member := range sm.partyMembers {
		for id, buff := range member.DesiredBuffs {
			if strings.ToLower(buff.SpellName) == spellName {
				delete(member.DesiredBuffs, id)
			}
		}
	}
}

// ClearDesiredBuffByStatusID clears a specific status ID across all members
func (sm *StatusMonitor) ClearDesiredBuffByStatusID(statusID int) {
	for _, member := range sm.partyMembers {
		delete(member.DesiredBuffs, statusID)
	}
}

// GetPartyCount returns the number of tracked party members
func (sm *StatusMonitor) GetPartyCount() int {
	return len(sm.partyMembers)
}

// SetHealthThresholds updates the health thresholds
func (sm *StatusMonitor) SetHealthThresholds(critical, low, medium int) {
	sm.healthThresholds.Critical = critical
	sm.healthThresholds.Low = low
	sm.healthThresholds.Medium = medium
}

// GetHealthThresholds returns the current health thresholds
func (sm *StatusMonitor) GetHealthThresholds() HealthThresholds {
	return sm.healthThresholds
}

// GetLastUpdateTime returns when the monitor was last updated
func (sm *StatusMonitor) GetLastUpdateTime() time.Time {
	return sm.lastUpdate
}

// GetBuffToStatusMap returns a mapping of common buff names/substrings to status IDs
func GetBuffToStatusMap() map[string]int {
	m := make(map[string]int)

	// Derive from status registry
	for id, info := range status.Registry {
		if info.IsBuff && info.NaSpell != "None" {
			// Use both the status name and the spell name as triggers
			m[strings.ToLower(info.Name)] = id
			m[strings.ToLower(info.NaSpell)] = id

			// Special case for names with spaces (also add version without space)
			if strings.Contains(info.Name, " ") {
				m[strings.ToLower(strings.ReplaceAll(info.Name, " ", ""))] = id
			}
			if strings.Contains(info.NaSpell, " ") {
				m[strings.ToLower(strings.ReplaceAll(info.NaSpell, " ", ""))] = id
			}
		}
	}

	return m
}

// GetElementalBarStatusMapping returns a mapping of elemental buff triggers to status IDs and spell names
func GetElementalBarStatusMapping() map[string]struct {
	StatusID  int
	SpellName string
} {
	return map[string]struct {
		StatusID  int
		SpellName string
	}{
		"firebuffs":    {100, "barfire"},
		"icebuffs":     {101, "barice"},
		"windbuffs":    {102, "barwind"},
		"earthbuffs":   {103, "barearth"},
		"thunderbuffs": {104, "barlighting"},
		"waterbuffs":   {105, "barwater"},
	}
}

// IsStale returns true if the monitor hasn't been updated recently
func (sm *StatusMonitor) IsStale() bool {
	return time.Since(sm.lastUpdate) > sm.updateInterval*2
}
