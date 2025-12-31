package casting

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"PandaBot/internal/buffSelector"
	"PandaBot/internal/cureSelector"
	"PandaBot/internal/entity"
	"PandaBot/internal/naSelector"
	"PandaBot/internal/spell"
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
}

// CastRequest represents a request to cast a spell
type CastRequest struct {
	ID        string
	Type      CastType
	SpellName string
	Target    string
	Priority  int
	Timeout   time.Duration
	Context   *CastContext
	Callback  CastCallback
}

// CastType defines the type of casting operation
type CastType int

const (
	CastTypeManual   CastType = iota // Manually specified spell
	CastTypeCure                     // Auto-selected cure spell
	CastTypeBuff                     // Auto-selected buff spell
	CastTypeNa                       // Auto-selected "na" spell
	CastTypeSequence                 // Multiple spells in sequence
	CastTypeItem                     // Use an item
	CastTypeProtect                  // Auto-selected Protect spell
	CastTypeShell                    // Auto-selected Shell spell
	CastTypeWhmPrep                  // WHM preparation sequence
	CastTypeReraise                  // Auto-selected Reraise spell
)

// CastContext provides context for spell selection and casting
type CastContext struct {
	CasterMP        int
	CasterJobLevels map[string]int
	CasterName      string // Name of the caster (for self-targeting spells)
	TargetEntity    *entity.Entity
	PartyMembers    []*entity.Entity
	PartySize       int
	StatusEffects   []int
	BuffType        string // For buff casting
	MissingHP       int    // For cure casting
}

// ActiveCast tracks an ongoing casting operation
type ActiveCast struct {
	Request           *CastRequest
	StartTime         time.Time
	State             CastState
	AttemptCount      int
	LastError         string
	SpellsInSequence  []string // For sequence casting
	CurrentSpellIndex int
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

// CastResult contains the result of a casting operation
type CastResult struct {
	Request   *CastRequest
	Success   bool
	Error     string
	Duration  time.Duration
	SpellCast string // The actual spell that was cast
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
		MPReservation: 0,                      // Keep 50 MP in reserve
		SequenceDelay: 500 * time.Millisecond, // Reduced since we check if ready
	}
}

// RequestCast submits a new casting request
func (ce *CastingEngine) RequestCast(request *CastRequest) error {
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

	// Check if we are already casting this spell on this target (prevent double triggers)
	for _, active := range ce.activeCasts {
		if active.Request.SpellName == request.SpellName &&
			active.Request.Target == request.Target &&
			(active.State == CastStatePending || active.State == CastStateInProgress) {
			ce.mu.Unlock()
			log.Printf("Ignoring duplicate cast request: %s on %s (already in state %s)",
				request.SpellName, request.Target, ce.castStateToString(active.State))
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

	// 1. Resolve spell selection based on cast type
	if err := ce.resolveSpellSelection(activeCast); err != nil {
		ce.mu.Unlock()
		return fmt.Errorf("spell selection failed: %v", err)
	}

	// 2. Check if we are already casting this spell on this target (prevent double triggers)
	// After resolution, request.SpellName should be populated for most types
	// If it's a sequence, we check the first spell
	for _, active := range ce.activeCasts {
		if active.Request.Target == request.Target &&
			(active.State == CastStatePending || active.State == CastStateInProgress) {

			// If both are single spells and match
			if ce.isEquivalentSpell(active.Request.SpellName, request.SpellName) {
				ce.mu.Unlock()
				log.Printf("Ignoring duplicate cast request: %s on %s (already in state %s)",
					request.SpellName, request.Target, ce.castStateToString(active.State))
				return nil
			}

			// If one or both are sequences, check if they overlap or are identical
			if (request.Type == CastTypeSequence || active.Request.Type == CastTypeSequence) &&
				active.Request.Type == request.Type &&
				ce.isEquivalentSpell(active.Request.SpellName, request.SpellName) {
				// For sequences of the same type starting with the same spell, assume duplicate
				ce.mu.Unlock()
				log.Printf("Ignoring duplicate sequence request: type %s, current spell %s on %s",
					ce.castTypeToString(request.Type), request.SpellName, request.Target)
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

// resolveSpellSelection determines the actual spell(s) to cast based on request type
func (ce *CastingEngine) resolveSpellSelection(activeCast *ActiveCast) error {
	request := activeCast.Request
	context := request.Context

	if context == nil {
		return fmt.Errorf("cast context is required")
	}

	switch request.Type {
	case CastTypeManual:
		// Spell already specified in request
		if request.SpellName == "" {
			return fmt.Errorf("spell name required for manual cast")
		}

	case CastTypeCure:
		log.Printf("[CASTING DEBUG] Resolving CastTypeCure spell selection")
		// Select optimal cure spell
		cureOption, err := ce.selectOptimalCure(context)
		if err != nil {
			log.Printf("[CASTING DEBUG] Cure selection failed: %v", err)
			return fmt.Errorf("cure selection failed: %v", err)
		}
		request.SpellName = cureOption.SpellName
		log.Printf("[CASTING DEBUG] Cure selection completed: %s", cureOption.SpellName)

	case CastTypeBuff:
		// Select optimal buff sequence
		buffSequence, err := ce.selectOptimalBuffs(context)
		if err != nil {
			return fmt.Errorf("buff selection failed: %v", err)
		}

		if len(buffSequence) == 1 {
			request.SpellName = buffSequence[0]
		} else {
			// Multiple spells - convert to sequence
			request.Type = CastTypeSequence
			activeCast.SpellsInSequence = buffSequence
			activeCast.CurrentSpellIndex = 0
			request.SpellName = buffSequence[0]
		}

	case CastTypeNa:
		// Select optimal "na" spell
		naSpell, err := ce.selectOptimalNaSpell(context)
		if err != nil {
			return fmt.Errorf("na spell selection failed: %v", err)
		}
		request.SpellName = naSpell

	case CastTypeSequence:
		// Spells already specified in SpellsInSequence
		if len(activeCast.SpellsInSequence) == 0 {
			return fmt.Errorf("spell sequence cannot be empty")
		}
		request.SpellName = activeCast.SpellsInSequence[0]

	case CastTypeItem:
		// Item usage - spell name is the item name
		if request.SpellName == "" {
			return fmt.Errorf("item name required for item cast")
		}

	case CastTypeProtect:
		// Select optimal Protect spell
		protectOption, err := ce.selectOptimalProtect(context)
		if err != nil {
			return fmt.Errorf("protect selection failed: %v", err)
		}
		request.SpellName = protectOption.SpellName

	case CastTypeShell:
		// Select optimal Shell spell
		shellOption, err := ce.selectOptimalShell(context)
		if err != nil {
			return fmt.Errorf("shell selection failed: %v", err)
		}
		request.SpellName = shellOption.SpellName

	case CastTypeReraise:
		// Select optimal Reraise spell
		reraiseOption, err := ce.buffSelector.SelectOptimalReraise(context.CasterJobLevels, context.CasterMP)
		if err != nil {
			return fmt.Errorf("reraise selection failed: %v", err)
		}
		request.SpellName = reraiseOption.SpellName

	case CastTypeWhmPrep:
		// Select WHM preparation sequence
		whmSequence := []string{}

		// 1. Light Arts (if available)
		if level, exists := context.CasterJobLevels["SCH"]; (exists && level >= 10) || (context.CasterJobLevels["WHM"] >= 20 && context.CasterJobLevels["SCH"] >= 10) {
			// Actually SCH main/sub 10.
			whmSequence = append(whmSequence, "Light Arts")
		} else if level, exists := context.CasterJobLevels["WHM"]; exists && level >= 1 {
			// If not SCH, check if WHM main has it? No, WHM doesn't have Light Arts, SCH does.
			// But if WHM/SCH, it's available at SCH 10.
		}

		// 2. Afflatus Solace (if available)
		if level, exists := context.CasterJobLevels["WHM"]; exists && level >= 40 {
			whmSequence = append(whmSequence, "Afflatus Solace")
		}

		// 3. Highest Reraise
		reraiseOption, err := ce.buffSelector.SelectOptimalReraise(context.CasterJobLevels, context.CasterMP)
		if err == nil {
			whmSequence = append(whmSequence, reraiseOption.SpellName)
		}

		// 4. Auspice
		if level, exists := context.CasterJobLevels["WHM"]; exists && level >= 50 {
			whmSequence = append(whmSequence, "Auspice")
		}

		if len(whmSequence) == 0 {
			return fmt.Errorf("no WHM prep actions available")
		}

		if len(whmSequence) == 1 {
			request.SpellName = whmSequence[0]
		} else {
			request.Type = CastTypeSequence
			activeCast.SpellsInSequence = whmSequence
			activeCast.CurrentSpellIndex = 0
			request.SpellName = whmSequence[0]
		}

	default:
		return fmt.Errorf("unknown cast type: %v", request.Type)
	}

	return nil
}

// selectOptimalCure selects the best cure spell for the context
func (ce *CastingEngine) selectOptimalCure(context *CastContext) (*cureSelector.CureOption, error) {
	log.Printf("[CURE DEBUG] Starting cure selection process")
	log.Printf("[CURE DEBUG] Caster MP: %d, MP Reservation: %d", context.CasterMP, ce.config.MPReservation)

	availableMP := context.CasterMP - ce.config.MPReservation
	if availableMP <= 0 {
		log.Printf("[CURE DEBUG] Insufficient MP: available=%d, reservation=%d", context.CasterMP, ce.config.MPReservation)
		return nil, fmt.Errorf("insufficient MP (need to reserve %d MP)", ce.config.MPReservation)
	}

	log.Printf("[CURE DEBUG] Available MP after reservation: %d", availableMP)

	// Determine if this is an emergency situation based on HP percentage
	prioritizeEfficiency := true // Default to efficiency mode
	if context.TargetEntity != nil && context.TargetEntity.HPPercent < 30 {
		// Emergency mode for critically low HP (< 30%)
		prioritizeEfficiency = false
		log.Printf("[CURE DEBUG] Emergency mode activated: target HP %d%% < 30%%", context.TargetEntity.HPPercent)
	} else {
		log.Printf("[CURE DEBUG] Efficiency mode: target HP %d%% >= 30%%",
			func() uint8 {
				if context.TargetEntity != nil {
					return context.TargetEntity.HPPercent
				} else {
					return 100
				}
			}())
	}

	if context.TargetEntity != nil {
		log.Printf("[CURE DEBUG] Target entity: %s, HP: %d/%d (%d%%), Job: %s Level: %d",
			context.TargetEntity.Name,
			context.TargetEntity.HPcurrent,
			context.TargetEntity.HPMax,
			context.TargetEntity.HPPercent,
			context.TargetEntity.Job,
			context.TargetEntity.JobLevel)

		// Calculate missing HP from actual values if available
		var actualMissingHP int
		if context.TargetEntity.HPMax > 0 && context.TargetEntity.HPcurrent <= context.TargetEntity.HPMax {
			actualMissingHP = int(context.TargetEntity.HPMax - context.TargetEntity.HPcurrent)
			log.Printf("[CURE DEBUG] Calculated missing HP from actual values: %d", actualMissingHP)
		} else {
			// Fall back to percentage calculation
			missingPercent := 100 - int(context.TargetEntity.HPPercent)
			estimatedMaxHP := 1000 // Default estimate
			actualMissingHP = (missingPercent * estimatedMaxHP) / 100
			log.Printf("[CURE DEBUG] Calculated missing HP from percentage: %d%% of %d = %d",
				missingPercent, estimatedMaxHP, actualMissingHP)
		}

		// Only proceed if there's actually missing HP
		if actualMissingHP <= 0 {
			log.Printf("[CURE DEBUG] Target doesn't need healing (missing HP: %d)", actualMissingHP)
			return nil, fmt.Errorf("target does not need healing")
		}

		option, err := ce.cureSelector.SelectOptimalCure(
			context.TargetEntity,
			context.PartyMembers,
			availableMP,
			context.CasterJobLevels,
			prioritizeEfficiency,
		)

		if err != nil {
			log.Printf("[CURE DEBUG] SelectOptimalCure failed: %v", err)
			return nil, err
		}

		log.Printf("[CURE DEBUG] Selected cure: %s (heal: %d, cost: %d MP, efficiency: %.2f)",
			option.SpellName, option.HealAmount, option.MPCost, option.Efficiency)

		return option, nil
	}

	if context.MissingHP > 0 {
		log.Printf("[CURE DEBUG] Using MissingHP context: %d HP", context.MissingHP)

		option, err := ce.cureSelector.SelectCureByDamage(
			context.MissingHP,
			availableMP,
			context.CasterJobLevels,
		)

		if err != nil {
			log.Printf("[CURE DEBUG] SelectCureByDamage failed: %v", err)
			return nil, err
		}

		log.Printf("[CURE DEBUG] Selected cure by damage: %s (heal: %d, cost: %d MP, efficiency: %.2f)",
			option.SpellName, option.HealAmount, option.MPCost, option.Efficiency)

		return option, nil
	}

	log.Printf("[CURE DEBUG] No valid cure target found")
	return nil, fmt.Errorf("no cure target or missing HP specified")
}

// selectOptimalBuffs selects the best buff spells for the context
func (ce *CastingEngine) selectOptimalBuffs(context *CastContext) ([]string, error) {
	availableMP := context.CasterMP - ce.config.MPReservation
	if availableMP <= 0 {
		return nil, fmt.Errorf("insufficient MP for buffs")
	}

	buffSequence, err := ce.buffSelector.GetOptimalBuffSequence(
		context.BuffType,
		context.CasterJobLevels,
		availableMP,
		context.PartySize,
	)
	if err != nil {
		return nil, err
	}

	// Convert buff options to spell names
	spellNames := make([]string, len(buffSequence))
	for i, buff := range buffSequence {
		spellNames[i] = buff.SpellName
	}

	return spellNames, nil
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
	)
}

// resolveSpellTarget determines the correct target for a spell based on its targeting requirements
func (ce *CastingEngine) resolveSpellTarget(spellName string, originalTarget string, context *CastContext) (string, error) {
	// For manual spells, we need to check the spell's targeting requirements
	// First try to get spell info from our selectors

	// Check if it's a cure spell
	if cureSpell, err := ce.cureSelector.GetCureSpellInfo(spellName); err == nil {
		return ce.resolveTargetByFlags(cureSpell.Targets, originalTarget, context)
	}

	// Check if it's a buff spell (including Bar spells)
	if buffSpell, err := ce.buffSelector.GetBuffSpellInfo(spellName); err == nil {
		return ce.resolveTargetByFlags(buffSpell.Targets, originalTarget, context)
	}

	// Check if it's a na spell
	if _, err := ce.naSelector.GetNaSpellInfo(spellName); err == nil {
		return originalTarget, nil
	}

	// Fallback to naming patterns for unknown spells
	if ce.isAreaSpellByName(spellName) {
		// Area spells must target the caster
		if context.CasterName != "" {
			return context.CasterName, nil
		}
		return "me", nil // Fallback
	}

	// Default to original target for single-target spells
	return originalTarget, nil
}

// resolveTargetByFlags resolves target based on spell target flags
func (ce *CastingEngine) resolveTargetByFlags(targetFlags spell.TargetFlags, originalTarget string, context *CastContext) (string, error) {
	// If spell can only target self, use caster name
	if targetFlags == spell.TargetSelf {
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
	return strings.HasSuffix(spellName, "ra") || // Protectra, Shellra, etc.
		strings.HasSuffix(spellName, "ra II") ||
		strings.HasSuffix(spellName, "ra III") ||
		strings.HasSuffix(spellName, "ra IV") ||
		strings.HasSuffix(spellName, "ra V") ||
		strings.Contains(spellName, "ga") // Curaga, etc.
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
func (ce *CastingEngine) selectOptimalNaSpell(context *CastContext) (string, error) {
	availableMP := context.CasterMP - ce.config.MPReservation
	if availableMP <= 0 {
		return "", fmt.Errorf("insufficient MP for na spell")
	}

	naOption, err := ce.naSelector.SelectOptimalNaSpell(
		context.StatusEffects,
		availableMP,
	)
	if err != nil {
		return "", err
	}

	return naOption.SpellName, nil
}

// processCast handles the casting process for an active cast
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

		// Execute the cast through the casting engine's internal logic
		success, err := ce.executeCast(activeCast)

		if success {
			// Cast command was sent successfully to the client
			// Now we wait for the client to report completion via NotifySpellComplete
			// The cast remains in CastStateInProgress until then
			log.Printf("Cast command sent successfully: %s -> %s", activeCast.Request.ID, activeCast.Request.SpellName)
			return
		}

		// Handle failure
		activeCast.LastError = err.Error()
		log.Printf("Cast execution failed: %s -> %s (error: %s)", activeCast.Request.ID, activeCast.Request.SpellName, err.Error())

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
		log.Printf("Cast failed after %d attempts: %s -> %s", ce.config.RetryAttempts, activeCast.Request.ID, activeCast.Request.SpellName)
		return
	}
}

// executeCast executes the actual spell cast (interface with game client)
func (ce *CastingEngine) executeCast(activeCast *ActiveCast) (bool, error) {
	request := activeCast.Request
	spellName := request.SpellName
	originalTarget := request.Target

	// Resolve the correct target for this spell
	resolvedTarget, err := ce.resolveSpellTarget(spellName, originalTarget, request.Context)
	if err != nil {
		return false, fmt.Errorf("failed to resolve target for spell %s: %v", spellName, err)
	}

	// Update the request with the resolved target
	request.Target = resolvedTarget

	// Log the casting attempt
	log.Printf("Executing cast: %s on %s (attempt %d)", spellName, resolvedTarget, activeCast.AttemptCount)

	// If we have a client manager, use it to execute the cast
	if ce.clientManager != nil {
		err := ce.clientManager.ExecuteCastRequest(request)
		if err != nil {
			return false, fmt.Errorf("failed to execute cast through client manager: %v", err)
		}
	} else {
		// Fallback for testing or when no client manager is available
		log.Printf("No client manager available, simulating cast execution")
	}

	// Return success - the actual completion will be reported by the client
	// For sequence casting, the next spell will be queued when this one completes
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
		result := &CastResult{
			Request:   activeCast.Request,
			Success:   activeCast.State == CastStateCompleted,
			Error:     activeCast.LastError,
			Duration:  record.Duration,
			SpellCast: activeCast.Request.SpellName,
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
	for id, cast := range ce.activeCasts {
		spellName := cast.Request.SpellName
		if spellName == "" && len(cast.SpellsInSequence) > 0 {
			if cast.CurrentSpellIndex < len(cast.SpellsInSequence) {
				spellName = cast.SpellsInSequence[cast.CurrentSpellIndex]
			} else {
				spellName = "SEQUENCE_DONE"
			}
		}
		log.Printf("    - %s: %s (State: %s, Priority: %d, Target: %s)",
			id, spellName, ce.castStateToString(cast.State),
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

// NotifySpellComplete notifies the engine that a spell has completed
// This is called by the client manager when a spell finishes casting
func (ce *CastingEngine) NotifySpellComplete(requestID string, success bool, errorMsg string) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	activeCast, exists := ce.activeCasts[requestID]
	if !exists {
		log.Printf("Received completion notification for unknown request: %s", requestID)
		//ce.logQueueState("COMPLETION_UNKNOWN_REQUEST", requestID)
		return
	}

	if !success {
		// Spell failed
		activeCast.State = CastStateFailed
		activeCast.LastError = errorMsg
		log.Printf("Spell failed: %s -> %s (error: %s)", requestID, activeCast.Request.SpellName, errorMsg)

		// Complete the cast since it failed
		go ce.completeCast(activeCast)
		return
	}

	// Spell succeeded - check if this is part of a sequence
	if activeCast.Request.Type == CastTypeSequence && activeCast.CurrentSpellIndex < len(activeCast.SpellsInSequence)-1 {
		// More spells in sequence - advance to the next one
		activeCast.CurrentSpellIndex++
		activeCast.Request.SpellName = activeCast.SpellsInSequence[activeCast.CurrentSpellIndex]

		log.Printf("Sequence spell completed, queuing next: %s (%d/%d) after %v delay",
			activeCast.Request.SpellName,
			activeCast.CurrentSpellIndex+1,
			len(activeCast.SpellsInSequence),
			ce.config.SequenceDelay)

		// Log queue state after sequence advancement
		//ce.logQueueState("SEQUENCE_ADVANCED", requestID)

		// Reset attempt count for the new spell
		activeCast.AttemptCount = 0
		activeCast.State = CastStatePending

		// Execute the next spell in the sequence after a small delay
		// Now that we have real-time ready checks, we can use a much smaller delay
		go func() {
			time.Sleep(500 * time.Millisecond)
			ce.processCast(activeCast)
		}()
		return
	}

	// Sequence complete or single spell - mark as completed
	activeCast.State = CastStateCompleted
	log.Printf("Spell sequence completed successfully: %s", requestID)

	// Final check: remove from active casts immediately after completion
	// to ensure it doesn't block the concurrent limit
	go ce.completeCast(activeCast)
}

// ResolveSpellTarget exposes the target resolution functionality for testing
func (ce *CastingEngine) ResolveSpellTarget(spellName string, originalTarget string, context *CastContext) (string, error) {
	return ce.resolveSpellTarget(spellName, originalTarget, context)
}

// SelectOptimalCure exposes the cure selection functionality for testing
func (ce *CastingEngine) SelectOptimalCure(context *CastContext) (*cureSelector.CureOption, error) {
	return ce.selectOptimalCure(context)
}

// GetStats returns casting engine statistics
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
