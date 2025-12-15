# Centralized Casting System

This package provides a centralized casting engine that consolidates all spell casting logic for PandaBot. It replaces the scattered casting logic that was previously distributed across multiple components.

## Overview

The centralized casting system consists of several key components:

### Core Components

1. **CastingEngine** (`casting.go`) - The main engine that manages casting requests, spell selection, and execution
2. **ClientInterface** (`client_interface.go`) - Defines how the casting engine communicates with game clients
3. **ServerAdapter** (`server_adapter.go`) - Integrates the casting system with the existing server infrastructure
4. **CastingHelper** (`convenience.go`) - Provides convenient methods for common casting operations

### Key Features

- **Unified Spell Selection**: Integrates CureSelector, BuffSelector, and NaSelector for optimal spell choice
- **Proper Target Resolution**: Automatically resolves correct targets based on FFXI spell mechanics
- **Priority-based Queuing**: Manages casting requests with priority levels and timeouts
- **Concurrent Cast Management**: Handles multiple simultaneous casting operations
- **Retry Logic**: Automatically retries failed casts with exponential backoff
- **Comprehensive Logging**: Tracks casting history and statistics
- **Flexible Configuration**: Customizable timeouts, retry attempts, and MP reservation

## Architecture

### Request Flow

1. **Request Creation**: A casting request is created with type, target, priority, and context
2. **Spell Selection**: The engine automatically selects optimal spells based on request type:
   - `CastTypeCure`: Uses CureSelector to find best healing spell
   - `CastTypeBuff`: Uses BuffSelector to determine buff sequence
   - `CastTypeNa`: Uses NaSelector to pick status removal spell
   - `CastTypeManual`: Uses explicitly specified spell
   - `CastTypeSequence`: Executes multiple spells in order
3. **Target Resolution**: Engine resolves correct target based on spell's targeting requirements:
   - `TargetSelf` spells (Protectra, Shellra, Curaga, Bar spells) → Target caster
   - `TargetPartyMember` spells (Cure, Protect, Shell, Na spells) → Target original recipient
4. **Client Selection**: ClientManager chooses appropriate client for execution
5. **Execution**: Command is sent to game client via existing protocol with resolved target
6. **Completion Tracking**: Engine monitors for success/failure notifications

### Cast Types

```go
const (
    CastTypeManual    // Manually specified spell
    CastTypeCure      // Auto-selected cure spell
    CastTypeBuff      // Auto-selected buff spell
    CastTypeNa        // Auto-selected "na" spell
    CastTypeSequence  // Multiple spells in sequence
)
```

### Configuration Options

```go
type CastingConfig struct {
    DefaultTimeout       time.Duration // Default request timeout
    MaxConcurrentCasts   int          // Maximum simultaneous casts
    RetryAttempts        int          // Number of retry attempts
    RetryDelay          time.Duration // Delay between retries
    PriorityThresholds  map[string]int // Priority level mappings
    MPReservation       int          // MP to keep in reserve
}
```

## Usage Examples

### Basic Cure Casting

```go
helper := casting.NewCastingHelper(engine, clientManager)

// Cast optimal cure for missing HP
requestID, err := helper.CastCureByDamage(
    "PlayerName",     // target
    500,              // missing HP
    200,              // available MP
    map[string]int{"WHM": 75}, // job levels
    8,                // priority
)
```

### Buff Casting

```go
// Cast fire resistance buffs
requestID, err := helper.CastBuffs(
    "PlayerName",     // target
    "firebuffs",      // buff type
    300,              // available MP
    map[string]int{"WHM": 60}, // job levels
    4,                // party size
    3,                // priority
)
```

### Status Removal

```go
// Remove paralysis and silence
requestID, err := helper.CastNaSpell(
    "PlayerName",     // target
    []int{4, 6},      // status effect IDs
    150,              // available MP
    map[string]int{"WHM": 40}, // job levels
    9,                // priority
)
```

### Manual Spell Casting

```go
// Cast specific spell
requestID, err := helper.CastSpell(
    "Cure III",       // spell name
    "PlayerName",     // target
    5,                // priority
    30*time.Second,   // timeout
)
```

### Sequence Casting

```go
// Cast multiple spells in order
requestID, err := helper.CastSpellSequence(
    []string{"Protect V", "Shell V", "Barfira"}, // spells
    "PlayerName",     // target
    3,                // priority
)
```

## Integration with Existing Server

The casting system integrates seamlessly with the existing server through the `CastingServerIntegration`:

```go
// In server initialization
server.castingSystem = casting.NewCastingServerIntegration()

// Register clients
server.castingSystem.RegisterClient(conn, playerName)

// Process triggers using centralized system
requestIDs := server.castingSystem.ProcessTriggerAction(
    sender, spells, target, priority, buffType)

// Handle spell completion
server.castingSystem.HandleSpellComplete(conn, commandID)
```

## Benefits

### Before Centralization
- Casting logic scattered across multiple files
- Inconsistent spell selection algorithms
- No unified priority system
- Limited retry and error handling
- Difficult to track casting statistics
- **Incorrect targeting**: Area spells could target wrong players

### After Centralization
- ✅ Single source of truth for all casting operations
- ✅ Consistent, optimal spell selection across all components
- ✅ **Proper FFXI targeting mechanics**: Self-targeting spells correctly target caster
- ✅ Unified priority-based queuing system
- ✅ Robust retry logic with exponential backoff
- ✅ Comprehensive casting statistics and history
- ✅ Easy to extend with new casting types
- ✅ Simplified testing and debugging

## Testing

The casting system includes comprehensive tests covering:

- Basic casting request handling
- Spell selection for all cast types
- Priority queuing and concurrent cast limits
- Configuration validation
- Error handling and retry logic

Run tests with:
```bash
go test ./internal/casting -v
```

## Future Enhancements

Potential improvements for the casting system:

1. **Advanced Client Selection**: Consider client location, MP levels, and current load
2. **Spell Cooldown Tracking**: Prevent casting spells still on cooldown
3. **Dynamic Priority Adjustment**: Adjust priorities based on combat state
4. **Casting Analytics**: Detailed performance metrics and optimization suggestions
5. **Plugin System**: Allow custom spell selection algorithms
6. **Batch Operations**: Optimize multiple casts for the same target

## Migration Guide

To migrate existing casting code to use the centralized system:

1. Replace direct spell selector calls with CastingHelper methods
2. Update trigger processing to use `ProcessTriggerAction`
3. Remove manual command queuing in favor of casting requests
4. Update spell completion handlers to notify the casting system
5. Replace scattered casting statistics with centralized metrics

The system is designed to be backward compatible, with fallback to legacy casting methods when needed.