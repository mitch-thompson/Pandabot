package casting

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"PandaBot/internal/action"
	"PandaBot/internal/buffSelector"
	"PandaBot/internal/config"
	"PandaBot/internal/cureSelector"
	"PandaBot/internal/entity"
	"PandaBot/internal/naSelector"
	"PandaBot/internal/player"
	"PandaBot/internal/registry"
)

// CastingEngine centralizes all spell casting logic and coordination
type CastingEngine struct {
	cureSelector *cureSelector.CureSelector
	buffSelector *buffSelector.BuffSelector
	naSelector   *naSelector.NaSpellSelector

	// Active casting state
	activeCasts map[string]*ActiveCast
	castHistory []*CastRecord
	mu          sync.RWMutex

	// Configuration
	config *CastingConfig

	// Client management
	clientManager *ClientManager

	// Player state
	Player *player.Player
}

// CastingConfig holds configuration for the casting engine
type CastingConfig struct {
	DefaultTimeout     time.Duration
	MaxConcurrentCasts int
	RetryAttempts      int
	RetryDelay         time.Duration
	PriorityThresholds map[string]int
	MPReservation      int           // MP to keep in reserve
	SequenceDelay      time.Duration // Delay between spells in a sequence
	IsPowerleveling    bool          // Whether PL mode is active
}

// CastRequest represents a request to execute an action
type CastRequest struct {
	ID       string
	Type     CastType
	Action   action.Actionable
	Target   string
	Priority int
	Timeout  time.Duration
	Context  *CastContext
	Callback CastCallback
}

// CastType defines the type of casting operation
type CastType int

const (
	CastTypeManual   CastType = iota // Manually specified action
	CastTypeCure                     // Auto-selected cure action
	CastTypeBuff                     // Auto-selected buff action
	CastTypeNa                       // Auto-selected "na" action
	CastTypeSequence                 // Multiple actions in sequence
	CastTypeItem                     // Use an item
	CastTypeProtect                  // Auto-selected Protect action
	CastTypeShell                    // Auto-selected Shell action
	CastTypeWhmPrep                  // WHM preparation sequence
	CastTypeReraise                  // Auto-selected Reraise action
	CastTypeRegen                    // Auto-selected Regen action
	CastTypeRefresh                  // Auto-selected Refresh action
)

// CastContext provides context for spell selection and casting
type CastContext struct {
	Player          *player.Player
	CasterMP        int
	CasterJobLevels map[string]int
	CasterName      string // Name of the caster (for self-targeting spells)
	TargetEntity    *entity.Entity
	PartyMembers    []*entity.Entity
	PartySize       int
	StatusEffects   []int
	BuffType        string // For buff casting
	MissingHP       int    // For cure casting
	OriginalTarget  string // Original target for sequences
	IsPowerleveling bool   // Whether PL mode is active
}

// ActiveCast tracks an ongoing casting operation
type ActiveCast struct {
	Request            *CastRequest
	StartTime          time.Time
	State              CastState
	AttemptCount       int
	LastError          string
	ActionsInSequence  []action.Actionable // For sequence casting
	CurrentActionIndex int
}

// CastState represents the state of a casting operation
type CastState int

const (
	CastStatePending CastState = iota
	CastStateInProgress
	CastStateCompleted
	CastStateFailed
	CastStateTimeout
	CastStateCancelled
)

// CastRecord keeps history of completed casts
type CastRecord struct {
	Request   *CastRequest
	StartTime time.Time
	EndTime   time.Time
	State     CastState
	Error     string
	Duration  time.Duration
}

// CastCallback is called when a cast completes or fails
type CastCallback func(result *CastResult)

// CastResult contains the result of an action operation
type CastResult struct {
	Request    *CastRequest
	Success    bool
	Error      string
	Duration   time.Duration
	ActionUsed string // The actual action that was used
}

// NewCastingEngine creates a new centralized casting engine
func NewCastingEngine(config *CastingConfig) *CastingEngine {
	if config == nil {
		config = DefaultCastingConfig()
	}

	return &CastingEngine{
		cureSelector: cureSelector.NewCureSelector(),
		buffSelector: buffSelector.NewBuffSelector(),
		naSelector:   naSelector.NewNaSpellSelector(),
		activeCasts:  make(map[string]*ActiveCast),
		castHistory:  make([]*CastRecord, 0),
		config:       config,
	}
}

// DefaultCastingConfig returns default configuration
func DefaultCastingConfig() *CastingConfig {
	cfg := config.Get()
	return &CastingConfig{
		DefaultTimeout:     30 * time.Second,
		MaxConcurrentCasts: 1,
		RetryAttempts:      30, // Increased to account for waiting on ready check
		RetryDelay:         1 * time.Second,
		PriorityThresholds: map[string]int{
			"critical": 9,
			"high":     7,
			"medium":   5,
			"low":      3,
		},
		MPReservation:   0,                      // Keep 50 MP in reserve
		SequenceDelay:   500 * time.Millisecond, // Reduced since we check if ready
		IsPowerleveling: cfg.IsPowerleveling,
	}
}

// RequestCast submits a new casting request
func (ce *CastingEngine) RequestCast(request *CastRequest) error {
	// Initialize context player if missing
	if request.Context != nil && request.Context.Player == nil {
		request.Context.Player = ce.Player
	}

	ce.mu.Lock()

	// If priority 10, cancel all other casts (Requirement 10.4)
	if request.Priority == 10 {
		for id, activeCast := range ce.activeCasts {
			activeCast.State = CastStateCancelled
			delete(ce.activeCasts, id)
		}
	}

	// Validate request
	if err := ce.validateRequest(request); err != nil {
		ce.mu.Unlock()
		return fmt.Errorf("invalid cast request: %v", err)
	}

	// Preserve original target in context for sequences
	if request.Context != nil && request.Context.OriginalTarget == "" {
		request.Context.OriginalTarget = request.Target
	}

	// Check concurrent cast limit
	if len(ce.activeCasts) >= ce.config.MaxConcurrentCasts {
		// Prune any terminal states that might have stuck in the map
		pruned := false
		for id, cast := range ce.activeCasts {
			if cast.State == CastStateCompleted || cast.State == CastStateFailed || cast.State == CastStateCancelled || cast.State == CastStateTimeout {
				delete(ce.activeCasts, id)
				pruned = true
			}
		}

		if pruned {
			log.Printf("[QUEUE DEBUG] Pruned terminal states from activeCasts map")
		}

		if len(ce.activeCasts) >= ce.config.MaxConcurrentCasts {
			ce.mu.Unlock()
			return fmt.Errorf("maximum concurrent casts reached (%d)", ce.config.MaxConcurrentCasts)
		}
	}

	// Check if we are already casting this action on this target (prevent double triggers)
	for _, active := range ce.activeCasts {
		if active.Request.Action != nil && request.Action != nil &&
			active.Request.Action.GetName() == request.Action.GetName() &&
			active.Request.Target == request.Target &&
			(active.State == CastStatePending || active.State == CastStateInProgress) {
			ce.mu.Unlock()
			log.Printf("Ignoring duplicate cast request: %s on %s (already in state %s)",
				request.Action.GetName(), request.Target, ce.castStateToString(active.State))
			return nil // Return nil as it's not an error, just redundant
		}
	}

	// Create active cast
	activeCast := &ActiveCast{
		Request:      request,
		StartTime:    time.Now(),
		State:        CastStatePending,
		AttemptCount: 0,
	}

	if err := ce.resolveActionSelection(activeCast); err != nil {
		ce.mu.Unlock()
		return fmt.Errorf("action selection failed: %v", err)
	}

	for _, active := range ce.activeCasts {
		if active.Request.Target == request.Target &&
			(active.State == CastStatePending || active.State == CastStateInProgress) {

			// If both have actions and names match
			if active.Request.Action != nil && request.Action != nil &&
				active.Request.Action.GetName() == request.Action.GetName() {
				ce.mu.Unlock()
				log.Printf("Ignoring duplicate cast request: %s on %s (already in state %s)",
					request.Action.GetName(), request.Target, ce.castStateToString(active.State))
				return nil
			}

			// If one or both are sequences, check if they overlap or are identical
			if (request.Type == CastTypeSequence || active.Request.Type == CastTypeSequence) &&
				active.Request.Type == request.Type &&
				active.Request.Action != nil && request.Action != nil &&
				active.Request.Action.GetName() == request.Action.GetName() {
				// For sequences of the same type starting with the same action, assume duplicate
				ce.mu.Unlock()
				log.Printf("Ignoring duplicate sequence request: type %s, current action %s on %s",
					ce.castTypeToString(request.Type), request.Action.GetName(), request.Target)
				return nil
			}
		}
	}

	// 3. Create active cast entry
	ce.activeCasts[request.ID] = activeCast
	//ce.logQueueState("CAST_ADDED", request.ID)
	ce.mu.Unlock()

	// Start casting process
	go ce.processCast(activeCast)

	return nil
}

// validateRequest validates a casting request
func (ce *CastingEngine) validateRequest(request *CastRequest) error {
	if request.ID == "" {
		return fmt.Errorf("request ID cannot be empty")
	}

	if request.Target == "" {
		return fmt.Errorf("target cannot be empty")
	}

	if request.Timeout == 0 {
		request.Timeout = ce.config.DefaultTimeout
	}

	// Check if request ID already exists
	if _, exists := ce.activeCasts[request.ID]; exists {
		return fmt.Errorf("request ID %s already exists", request.ID)
	}

	return nil
}

// resolveActionSelection determines the actual action(s) to use based on request type
func (ce *CastingEngine) resolveActionSelection(activeCast *ActiveCast) error {
	request := activeCast.Request
	context := request.Context

	if context == nil {
		return fmt.Errorf("cast context is required")
	}

	switch request.Type {
	case CastTypeManual:
		// Action already specified in request
		if request.Action == nil {
			return fmt.Errorf("action required for manual cast")
		}

	case CastTypeCure:
		// Select optimal cure action
		cureOption, err := ce.selectOptimalCure(context)
		if err != nil {
			log.Printf("[CASTING DEBUG] Cure selection failed: %v", err)
			return fmt.Errorf("cure selection failed: %v", err)
		}
		s, err := registry.GetSpell(cureOption.SpellName)
		if err != nil {
			return err
		}
		request.Action = s

	case CastTypeBuff:
		// Select optimal buff sequence
		buffSequence, err := ce.selectOptimalBuffs(context)
		if err != nil {
			return fmt.Errorf("buff selection failed: %v", err)
		}

		if len(buffSequence) == 1 {
			request.Action = buffSequence[0]
		} else {
			// Multiple actions - convert to sequence
			request.Type = CastTypeSequence
			activeCast.ActionsInSequence = buffSequence
			activeCast.CurrentActionIndex = 0
			request.Action = buffSequence[0]
		}

	case CastTypeNa:
		// Select optimal "na" action
		naAction, err := ce.selectOptimalNaSpell(context)
		if err != nil {
			return fmt.Errorf("na action selection failed: %v", err)
		}
		request.Action = naAction

	case CastTypeSequence:
		// Actions already specified in ActionsInSequence
		if len(activeCast.ActionsInSequence) == 0 {
			return fmt.Errorf("action sequence cannot be empty")
		}
		request.Action = activeCast.ActionsInSequence[0]

	case CastTypeItem:
		// Item usage
		if request.Action == nil {
			return fmt.Errorf("item action required for item cast")
		}

	case CastTypeProtect:
		// Select optimal Protect action
		protectOption, err := ce.selectOptimalProtect(context)
		if err != nil {
			return fmt.Errorf("protect selection failed: %v", err)
		}
		s, err := registry.GetSpell(protectOption.SpellName)
		if err != nil {
			return err
		}
		request.Action = s

	case CastTypeShell:
		// Select optimal Shell action
		shellOption, err := ce.selectOptimalShell(context)
		if err != nil {
			return fmt.Errorf("shell selection failed: %v", err)
		}
		s, err := registry.GetSpell(shellOption.SpellName)
		if err != nil {
			return err
		}
		request.Action = s

	case CastTypeReraise:
		// Select optimal Reraise action
		reraiseOption, err := ce.buffSelector.SelectOptimalReraise(context.CasterJobLevels, context.CasterMP, context.Player)
		if err != nil {
			return fmt.Errorf("reraise selection failed: %v", err)
		}
		s, err := registry.GetSpell(reraiseOption.SpellName)
		if err != nil {
			return err
		}
		request.Action = s

	case CastTypeRegen:
		// Select optimal Regen action
		regenOption, err := ce.buffSelector.SelectOptimalRegen(context.CasterJobLevels, context.CasterMP, context.Player)
		if err != nil {
			return fmt.Errorf("regen selection failed: %v", err)
		}
		s, err := registry.GetSpell(regenOption.SpellName)
		if err != nil {
			return err
		}
		request.Action = s

	case CastTypeRefresh:
		// Select optimal Refresh action
		refreshOption, err := ce.buffSelector.SelectOptimalRefresh(context.CasterJobLevels, context.CasterMP, context.Player)
		if err != nil {
			return fmt.Errorf("refresh selection failed: %v", err)
		}
		s, err := registry.GetSpell(refreshOption.SpellName)
		if err != nil {
			return err
		}
		request.Action = s

	case CastTypeWhmPrep:
		whmSequence := []action.Actionable{}

		if level, exists := context.CasterJobLevels["SCH"]; exists && level >= 10 {
			if a, err := registry.GetAbility("Light Arts"); err == nil {
				whmSequence = append(whmSequence, a)
			}
		}

		if level, exists := context.CasterJobLevels["WHM"]; exists && level >= 40 {
			if a, err := registry.GetAbility("Afflatus Solace"); err == nil {
				whmSequence = append(whmSequence, a)
			}
		}

		reraiseOption, err := ce.buffSelector.SelectOptimalReraise(context.CasterJobLevels, context.CasterMP, context.Player)
		if err == nil {
			if s, err := registry.GetSpell(reraiseOption.SpellName); err == nil {
				whmSequence = append(whmSequence, s)
			}
		}

		// Add Auspice if WHM 50+
		if level, exists := context.CasterJobLevels["WHM"]; exists && level >= 50 {
			if s, err := registry.GetSpell("Auspice"); err == nil {
				// Check recast for Auspice
				if context.Player == nil || context.Player.CanCast("Auspice") {
					whmSequence = append(whmSequence, s)
				}
			}
		}

		if len(whmSequence) == 0 {
			return fmt.Errorf("no WHM prep actions available")
		}

		if len(whmSequence) == 1 {
			request.Action = whmSequence[0]
		} else {
			request.Type = CastTypeSequence
			activeCast.ActionsInSequence = whmSequence
			activeCast.CurrentActionIndex = 0
			request.Action = whmSequence[0]
		}

	default:
		return fmt.Errorf("unknown cast type: %v", request.Type)
	}

	return nil
}

// selectOptimalCure selects the best cure spell for the context
func (ce *CastingEngine) selectOptimalCure(context *CastContext) (*cureSelector.CureOption, error) {
	availableMP := context.CasterMP - ce.config.MPReservation
	if availableMP <= 0 {
		return nil, fmt.Errorf("insufficient MP (need to reserve %d MP)", ce.config.MPReservation)
	}
	// Determine if this is an emergency situation based on HP percentage
	prioritizeEfficiency := true                                            // Default to efficiency mode
	if context.TargetEntity != nil && context.TargetEntity.HPPercent < 30 { //todo this is in configs
		// Emergency mode for critically low HP (< 30%)
		prioritizeEfficiency = false
	}

	if context.TargetEntity != nil {
		// Calculate missing HP from actual values if available
		var actualMissingHP int
		if context.TargetEntity.HPMax > 0 && context.TargetEntity.HPcurrent <= context.TargetEntity.HPMax {
			actualMissingHP = int(context.TargetEntity.HPMax - context.TargetEntity.HPcurrent)
		} else {
			// Fall back to percentage calculation
			missingPercent := 100 - int(context.TargetEntity.HPPercent)
			estimatedMaxHP := 1000 // Default estimate
			actualMissingHP = (missingPercent * estimatedMaxHP) / 100
		}

		// Only proceed if there's actually missing HP
		if actualMissingHP <= 0 {
			return nil, fmt.Errorf("target does not need healing")
		}

		option, err := ce.cureSelector.SelectOptimalCure(
			context.TargetEntity,
			context.PartyMembers,
			availableMP,
			context.CasterJobLevels,
			prioritizeEfficiency,
			context.Player,
			context.IsPowerleveling,
		)

		if err != nil {
			return nil, err
		}
		return option, nil
	}

	if context.MissingHP > 0 {
		option, err := ce.cureSelector.SelectCureByDamage(
			context.MissingHP,
			availableMP,
			context.CasterJobLevels,
			context.Player,
			context.IsPowerleveling,
		)

		if err != nil {
			return nil, err
		}

		return option, nil
	}
	return nil, fmt.Errorf("no cure target or missing HP specified")
}

// selectOptimalBuffs selects the best buff spells for the context
func (ce *CastingEngine) selectOptimalBuffs(context *CastContext) ([]action.Actionable, error) {
	availableMP := context.CasterMP - ce.config.MPReservation
	if availableMP <= 0 {
		return nil, fmt.Errorf("insufficient MP for buffs")
	}

	buffSequence, err := ce.buffSelector.GetOptimalBuffSequence(
		context.BuffType,
		context.CasterJobLevels,
		availableMP,
		context.PartySize,
		context.Player,
		context.IsPowerleveling,
	)
	if err != nil {
		return nil, err
	}

	// Convert buff options to actionable items
	actions := make([]action.Actionable, len(buffSequence))
	for i, buff := range buffSequence {
		s, err := registry.GetSpell(buff.SpellName)
		if err != nil {
			return nil, fmt.Errorf("buff spell %s not found in registry", buff.SpellName)
		}
		actions[i] = s
	}

	return actions, nil
}

// selectOptimalProtect selects the best Protect spell for the context
func (ce *CastingEngine) selectOptimalProtect(context *CastContext) (*buffSelector.BuffOption, error) {
	availableMP := context.CasterMP - ce.config.MPReservation
	if availableMP <= 0 {
		return nil, fmt.Errorf("insufficient MP for Protect")
	}

	return ce.buffSelector.SelectOptimalProtect(
		context.CasterJobLevels,
		availableMP,
		context.PartySize,
		context.Player,
		context.IsPowerleveling,
	)
}

// selectOptimalShell selects the best Shell spell for the context
func (ce *CastingEngine) selectOptimalShell(context *CastContext) (*buffSelector.BuffOption, error) {
	availableMP := context.CasterMP - ce.config.MPReservation
	if availableMP <= 0 {
		return nil, fmt.Errorf("insufficient MP for Shell")
	}

	return ce.buffSelector.SelectOptimalShell(
		context.CasterJobLevels,
		availableMP,
		context.PartySize,
		context.Player,
		context.IsPowerleveling,
	)
}

// resolveActionTarget determines the correct target for an action based on its targeting requirements
func (ce *CastingEngine) resolveActionTarget(actionName string, originalTarget string, context *CastContext) (string, error) {
	// If it's a known spell, resolve using its flags
	if s, err := registry.GetSpell(actionName); err == nil {
		return ce.resolveTargetByFlags(s.GetTargetFlags(), originalTarget, context)
	}

	// If it's a known ability, resolve using its flags
	if a, err := registry.GetAbility(actionName); err == nil {
		return ce.resolveTargetByFlags(a.GetTargetFlags(), originalTarget, context)
	}

	// If it's a known item, resolve using its flags
	if i, err := registry.GetItem(actionName); err == nil {
		return ce.resolveTargetByFlags(i.GetTargetFlags(), originalTarget, context)
	}

	// Fallback to naming patterns for unknown actions
	lowerName := strings.ToLower(actionName)
	if ce.isAreaSpellByName(actionName) ||
		lowerName == "light arts" || lowerName == "dark arts" ||
		lowerName == "afflatus solace" || lowerName == "afflatus misery" ||
		lowerName == "auspice" || strings.Contains(lowerName, "reraise") {
		// These must target the caster
		if context.CasterName != "" {
			return context.CasterName, nil
		}
		return "me", nil // Fallback
	}

	// Default to original target for single-target actions
	return originalTarget, nil
}

// resolveTargetByFlags resolves target based on spell target flags
func (ce *CastingEngine) resolveTargetByFlags(targetFlags action.TargetFlags, originalTarget string, context *CastContext) (string, error) {
	// If spell can ONLY target self (and not other players), use caster name
	// TargetSelf is often combined with other flags for spells that CAN target others but can also be cast on self.
	// But in FFXI, area spells like Protectra have ONLY TargetSelf (and maybe TargetAoE).
	// Single target spells have TargetSelf | TargetPartyMember | TargetPlayer.

	// A spell is "self-only" if it HAS TargetSelf AND NOT (TargetPartyMember OR TargetPlayer OR TargetEnemy)
	isSelfOnly := (targetFlags&action.TargetSelf != 0) && (targetFlags&(action.TargetPartyMember|action.TargetPlayer|action.TargetEnemy) == 0)

	if isSelfOnly {
		if context.CasterName != "" {
			return context.CasterName, nil
		}
		return "me", nil
	}

	// For other targeting types, use the original target
	// TODO: Add validation that the target is valid for the spell type
	return originalTarget, nil
}

// isAreaSpellByName checks if a spell is an area spell based on naming patterns
// This is a fallback method and should be replaced with proper spell metadata lookup
func (ce *CastingEngine) isAreaSpellByName(spellName string) bool {
	// Area spells in FFXI have predictable naming patterns
	lowerName := strings.ToLower(spellName)
	return strings.HasSuffix(lowerName, "ra") || // Protectra, Shellra, etc.
		strings.HasSuffix(lowerName, "ra ii") ||
		strings.HasSuffix(lowerName, "ra iii") ||
		strings.HasSuffix(lowerName, "ra iv") ||
		strings.HasSuffix(lowerName, "ra v") ||
		strings.Contains(lowerName, "ga") // Curaga, etc.
}

// isEquivalentSpell checks if two spell names refer to the same effect
func (ce *CastingEngine) isEquivalentSpell(spell1, spell2 string) bool {
	if spell1 == spell2 {
		return true
	}

	// Normalize by removing "ra" suffix and Roman numerals
	normalize := func(s string) string {
		s = strings.Replace(s, "ra", "", 1)

		// Remove Roman numerals (I, II, III, IV, V)
		parts := strings.Split(s, " ")
		if len(parts) > 1 {
			lastPart := parts[len(parts)-1]
			isRoman := true
			for _, char := range lastPart {
				if char != 'I' && char != 'V' && char != 'X' {
					isRoman = false
					break
				}
			}
			if isRoman {
				return strings.Join(parts[:len(parts)-1], " ")
			}
		}
		return s
	}

	return normalize(spell1) == normalize(spell2)
}

// selectOptimalNaSpell selects the best "na" spell for the context
func (ce *CastingEngine) selectOptimalNaSpell(context *CastContext) (action.Actionable, error) {
	if context.IsPowerleveling {
		return nil, fmt.Errorf("na spells and erase are disabled in powerleveling mode")
	}

	availableMP := context.CasterMP - ce.config.MPReservation
	if availableMP <= 0 {
		return nil, fmt.Errorf("insufficient MP for na spell")
	}

	naOption, err := ce.naSelector.SelectOptimalNaSpell(
		context.StatusEffects,
		availableMP,
		context.CasterJobLevels,
		context.Player,
	)
	if err != nil {
		return nil, err
	}

	s, err := registry.GetSpell(naOption.SpellName)
	if err != nil {
		return nil, fmt.Errorf("na spell %s not found in registry", naOption.SpellName)
	}
	return s, nil
}

// processCast handles the action execution process for an active cast
func (ce *CastingEngine) processCast(activeCast *ActiveCast) {
	for activeCast.AttemptCount < ce.config.RetryAttempts {
		// Check if cast was cancelled before starting/retrying
		ce.mu.RLock()
		if activeCast.State == CastStateCancelled {
			ce.mu.RUnlock()
			log.Printf("Cast process aborted for %s (cancelled)", activeCast.Request.ID)
			return
		}
		ce.mu.RUnlock()

		activeCast.AttemptCount++

		// Update state
		ce.mu.Lock()
		activeCast.State = CastStateInProgress
		//ce.logQueueState("CAST_IN_PROGRESS", activeCast.Request.ID)
		ce.mu.Unlock()

		// Execute the action through the casting engine's internal logic
		success, err := ce.executeCast(activeCast)

		if success {
			// Action command was sent successfully to the client
			// Now we wait for the client to report completion via NotifyActionComplete
			// The cast remains in CastStateInProgress until then
			actionName := "unknown"
			if activeCast.Request.Action != nil {
				actionName = activeCast.Request.Action.GetName()
			}
			log.Printf("Action command sent successfully: %s -> %s", activeCast.Request.ID, actionName)
			return
		}

		// Handle failure
		activeCast.LastError = err.Error()
		actionName := "unknown"
		if activeCast.Request.Action != nil {
			actionName = activeCast.Request.Action.GetName()
		}
		log.Printf("Action execution failed: %s -> %s (error: %s)", activeCast.Request.ID, actionName, err.Error())

		// Check if we should retry
		if activeCast.AttemptCount < ce.config.RetryAttempts {
			// If not ready, wait a bit longer or use standard retry delay
			delay := ce.config.RetryDelay
			if strings.Contains(activeCast.LastError, "client not ready") {
				// If client is just busy (casting/moving), we can retry sooner or later
				// but let's stick to config or maybe a shorter 1s delay
				delay = 1 * time.Second
			} else if strings.Contains(activeCast.LastError, "disconnected") || strings.Contains(activeCast.LastError, "closed network connection") {
				// If client is disconnected, we should retry sooner to pick a different client
				// or wait for reconnection
				delay = 2 * time.Second
			}
			time.Sleep(delay)
			continue
		}

		// All attempts failed
		ce.mu.Lock()
		activeCast.State = CastStateFailed
		ce.mu.Unlock()
		log.Printf("Action failed after %d attempts: %s -> %s", ce.config.RetryAttempts, activeCast.Request.ID, actionName)
		return
	}
}

// executeCast executes the actual action (interface with game client)
func (ce *CastingEngine) executeCast(activeCast *ActiveCast) (bool, error) {
	request := activeCast.Request
	if request.Action == nil {
		return false, fmt.Errorf("no action specified in request")
	}

	actionName := request.Action.GetName()
	originalTarget := request.Target

	// Use original target from context if it's a sequence and we might have modified request.Target in a previous step
	if request.Type == CastTypeSequence && request.Context.OriginalTarget != "" {
		originalTarget = request.Context.OriginalTarget
	}

	// Resolve the correct target for this action
	resolvedTarget, err := ce.resolveActionTarget(actionName, originalTarget, request.Context)
	if err != nil {
		return false, fmt.Errorf("failed to resolve target for action %s: %v", actionName, err)
	}

	// Update the request with the resolved target for the current execution
	// But ONLY if it's different, to avoid unnecessary updates if we're debugging
	if request.Target != resolvedTarget {
		request.Target = resolvedTarget
	}

	// Log the execution attempt
	log.Printf("Executing action: %s on %s (attempt %d)", actionName, resolvedTarget, activeCast.AttemptCount)

	// If we have a client manager, use it to execute the request
	if ce.clientManager != nil {
		err := ce.clientManager.ExecuteCastRequest(request)
		if err != nil {
			return false, fmt.Errorf("failed to execute action through client manager: %v", err)
		}
	} else {
		// Fallback for testing or when no client manager is available
		log.Printf("No client manager available, simulating action execution")
	}

	// Return success - the actual completion will be reported by the client
	// For sequence casting, the next action will be queued when this one completes
	return true, nil
}

// completeCast finalizes a casting operation
func (ce *CastingEngine) completeCast(activeCast *ActiveCast) {
	ce.mu.Lock()
	// Check if it's already removed (might happen due to pruning or multiple completion calls)
	if _, exists := ce.activeCasts[activeCast.Request.ID]; !exists {
		ce.mu.Unlock()
		return
	}

	// Remove from active casts
	delete(ce.activeCasts, activeCast.Request.ID)

	// Set recast timer if the action was a spell and it was successful
	if activeCast.State == CastStateCompleted && activeCast.Request.Action != nil {
		if activeCast.Request.Action.GetActionType() == action.ActionTypeSpell {
			// Try to get as Spell if possible (or look up in registry)
			s, err := registry.GetSpell(activeCast.Request.Action.GetName())
			if err == nil && s.Recast > 0 && activeCast.Request.Context.Player != nil {
				recastDuration := time.Duration(s.Recast * float32(time.Second))
				readyAt := time.Now().Add(recastDuration)
				activeCast.Request.Context.Player.SetSpellRecast(s.ID, readyAt)
			}
		}
	}

	// Log queue state after removal
	//ce.logQueueState("CAST_REMOVED", activeCast.Request.ID)
	ce.mu.Unlock()

	// Create cast record
	endTime := time.Now()
	record := &CastRecord{
		Request:   activeCast.Request,
		StartTime: activeCast.StartTime,
		EndTime:   endTime,
		State:     activeCast.State,
		Error:     activeCast.LastError,
		Duration:  endTime.Sub(activeCast.StartTime),
	}

	// Add to history (keep last 100 records)
	ce.mu.Lock()
	ce.castHistory = append(ce.castHistory, record)
	if len(ce.castHistory) > 100 {
		ce.castHistory = ce.castHistory[1:]
	}
	ce.mu.Unlock()

	// Call callback if provided
	if activeCast.Request.Callback != nil {
		actionName := "unknown"
		if activeCast.Request.Action != nil {
			actionName = activeCast.Request.Action.GetName()
		}

		result := &CastResult{
			Request:    activeCast.Request,
			Success:    activeCast.State == CastStateCompleted,
			Error:      activeCast.LastError,
			Duration:   record.Duration,
			ActionUsed: actionName,
		}

		go activeCast.Request.Callback(result)
	}
}

// CancelCast cancels an active casting operation
func (ce *CastingEngine) CancelCast(requestID string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	activeCast, exists := ce.activeCasts[requestID]
	if !exists {
		return fmt.Errorf("cast request %s not found", requestID)
	}

	activeCast.State = CastStateCancelled

	// Log queue state after cancellation
	//ce.logQueueState("CAST_CANCELLED", requestID)

	return nil
}

// ClearQueue cancels all active and pending casting operations
func (ce *CastingEngine) ClearQueue() {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	for id, activeCast := range ce.activeCasts {
		if activeCast.State == CastStatePending || activeCast.State == CastStateInProgress {
			activeCast.State = CastStateCancelled
			log.Printf("Cancelled cast request %s due to queue clear", id)
		}
		delete(ce.activeCasts, id)
	}
}

// GetActiveCasts returns information about currently active casts
func (ce *CastingEngine) GetActiveCasts() map[string]*ActiveCast {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[string]*ActiveCast)
	for id, cast := range ce.activeCasts {
		result[id] = cast
	}

	return result
}

// GetCastHistory returns recent casting history
func (ce *CastingEngine) GetCastHistory(limit int) []*CastRecord {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	if limit <= 0 || limit > len(ce.castHistory) {
		limit = len(ce.castHistory)
	}

	// Return the last 'limit' records
	start := len(ce.castHistory) - limit
	result := make([]*CastRecord, limit)
	copy(result, ce.castHistory[start:])

	return result
}

// SetClientManager sets the client manager for spell execution
func (ce *CastingEngine) SetClientManager(clientManager *ClientManager) {
	ce.clientManager = clientManager
}

// logQueueState logs the current state of the casting queue for debugging
// NOTE: This function assumes the caller already holds the mutex lock
func (ce *CastingEngine) logQueueState(operation string, requestID string) {
	log.Printf("[QUEUE DEBUG] %s - RequestID: %s", operation, requestID)
	log.Printf("  Active casts (%d):", len(ce.activeCasts))
	for _, cast := range ce.activeCasts {
		actionName := "unknown"
		if cast.Request.Action != nil {
			actionName = cast.Request.Action.GetName()
		} else if len(cast.ActionsInSequence) > 0 {
			if cast.CurrentActionIndex < len(cast.ActionsInSequence) {
				actionName = cast.ActionsInSequence[cast.CurrentActionIndex].GetName()
			} else {
				actionName = "SEQUENCE_DONE"
			}
		}
		log.Printf("    - %s: %s (State: %s, Priority: %d, Target: %s)",
			cast.Request.ID, actionName, ce.castStateToString(cast.State),
			cast.Request.Priority, cast.Request.Target)
	}
}

// castStateToString converts CastState to readable string
func (ce *CastingEngine) castStateToString(state CastState) string {
	switch state {
	case CastStatePending:
		return "PENDING"
	case CastStateInProgress:
		return "IN_PROGRESS"
	case CastStateCompleted:
		return "COMPLETED"
	case CastStateFailed:
		return "FAILED"
	case CastStateTimeout:
		return "TIMEOUT"
	case CastStateCancelled:
		return "CANCELLED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int(state))
	}
}

// castTypeToString converts CastType to readable string
func (ce *CastingEngine) castTypeToString(castType CastType) string {
	switch castType {
	case CastTypeManual:
		return "MANUAL"
	case CastTypeCure:
		return "CURE"
	case CastTypeBuff:
		return "BUFF"
	case CastTypeNa:
		return "NA"
	case CastTypeSequence:
		return "SEQUENCE"
	case CastTypeReraise:
		return "RERAISE"
	case CastTypeWhmPrep:
		return "WHMPREP"
	case CastTypeProtect:
		return "PROTECT"
	case CastTypeShell:
		return "SHELL"
	case CastTypeItem:
		return "ITEM"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int(castType))
	}
}

// NotifyActionComplete notifies the engine that an action has completed
// This is called by the client manager when an action finishes
func (ce *CastingEngine) NotifyActionComplete(requestID string, success bool, errorMsg string) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	activeCast, exists := ce.activeCasts[requestID]
	if !exists {
		log.Printf("Received completion notification for unknown request: %s", requestID)
		return
	}

	if !success {
		// Action failed
		activeCast.State = CastStateFailed
		activeCast.LastError = errorMsg
		actionName := "unknown"
		if activeCast.Request.Action != nil {
			actionName = activeCast.Request.Action.GetName()
		}
		log.Printf("Action failed: %s -> %s (error: %s)", requestID, actionName, errorMsg)

		// Complete the cast since it failed
		go ce.completeCast(activeCast)
		return
	}

	// Action succeeded - check if this is part of a sequence
	if activeCast.Request.Type == CastTypeSequence && activeCast.CurrentActionIndex < len(activeCast.ActionsInSequence)-1 {
		// More actions in sequence - advance to the next one
		activeCast.CurrentActionIndex++
		activeCast.Request.Action = activeCast.ActionsInSequence[activeCast.CurrentActionIndex]

		log.Printf("Sequence action completed, queuing next: %s (%d/%d) after %v delay",
			activeCast.Request.Action.GetName(),
			activeCast.CurrentActionIndex+1,
			len(activeCast.ActionsInSequence),
			ce.config.SequenceDelay)

		// Reset attempt count for the new action
		activeCast.AttemptCount = 0
		activeCast.State = CastStatePending

		// Execute the next action in the sequence after a small delay
		go func() {
			time.Sleep(500 * time.Millisecond)
			ce.processCast(activeCast)
		}()
		return
	}

	// Sequence complete or single action - mark as completed
	activeCast.State = CastStateCompleted
	log.Printf("Action sequence completed successfully: %s", requestID)

	go ce.completeCast(activeCast)
}

// ResolveActionTarget exposes the target resolution functionality for testing
func (ce *CastingEngine) ResolveActionTarget(actionName string, originalTarget string, context *CastContext) (string, error) {
	return ce.resolveActionTarget(actionName, originalTarget, context)
}

// SelectOptimalCure exposes the cure selection functionality for testing
func (ce *CastingEngine) SelectOptimalCure(context *CastContext) (*cureSelector.CureOption, error) {
	return ce.selectOptimalCure(context)
}

func (ce *CastingEngine) SelectOptimalNaAction(context *CastContext) (action.Actionable, error) {
	return ce.selectOptimalNaSpell(context)
}

// GetStats returns casting engine statistics
func (ce *CastingEngine) SelectOptimalProtect(context *CastContext) (*buffSelector.BuffOption, error) {
	return ce.selectOptimalProtect(context)
}

func (ce *CastingEngine) SelectOptimalShell(context *CastContext) (*buffSelector.BuffOption, error) {
	return ce.selectOptimalShell(context)
}

func (ce *CastingEngine) SelectOptimalReraise(jobLevels map[string]int, availableMP int, p *player.Player) (*buffSelector.BuffOption, error) {
	availableMP = availableMP - ce.config.MPReservation
	if availableMP <= 0 {
		return nil, fmt.Errorf("insufficient MP for Reraise")
	}

	return ce.buffSelector.SelectOptimalReraise(jobLevels, availableMP, p)
}

// SelectOptimalRegen selects the highest Regen spell available
func (ce *CastingEngine) SelectOptimalRegen(jobLevels map[string]int, availableMP int, p *player.Player) (*buffSelector.BuffOption, error) {
	availableMP = availableMP - ce.config.MPReservation
	if availableMP <= 0 {
		return nil, fmt.Errorf("insufficient MP for Regen")
	}

	return ce.buffSelector.SelectOptimalRegen(jobLevels, availableMP, p)
}

func (ce *CastingEngine) SelectOptimalRefresh(jobLevels map[string]int, availableMP int, p *player.Player) (*buffSelector.BuffOption, error) {
	availableMP = availableMP - ce.config.MPReservation
	if availableMP <= 0 {
		return nil, fmt.Errorf("insufficient MP for Refresh")
	}

	return ce.buffSelector.SelectOptimalRefresh(jobLevels, availableMP, p)
}

func (ce *CastingEngine) GetCastingEngine() *CastingEngine {
	return ce
}

// GetPlayer returns the player object for recast checks
func (ce *CastingEngine) GetPlayer() *player.Player {
	return ce.Player
}

func (ce *CastingEngine) GetStats() map[string]interface{} {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	// Count states
	stateCounts := make(map[CastState]int)
	for _, cast := range ce.activeCasts {
		stateCounts[cast.State]++
	}

	// Calculate success rate from history
	var totalCasts, successfulCasts int
	for _, record := range ce.castHistory {
		totalCasts++
		if record.State == CastStateCompleted {
			successfulCasts++
		}
	}

	successRate := 0.0
	if totalCasts > 0 {
		successRate = float64(successfulCasts) / float64(totalCasts) * 100
	}

	return map[string]interface{}{
		"active_casts":   len(ce.activeCasts),
		"state_counts":   stateCounts,
		"total_history":  len(ce.castHistory),
		"success_rate":   successRate,
		"max_concurrent": ce.config.MaxConcurrentCasts,
	}
}
