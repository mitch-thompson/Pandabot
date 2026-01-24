# Unified Actions Design & Planning

## Problem Description
The application currently handles spells, abilities, and items inconsistently. 
- Spells use a robust `CastingEngine` and `SpellCommand`.
- Abilities and Items are partially supported or use fragmented logic.
- Definitions are scattered across various selectors (`cureSelector`, `buffSelector`, etc.).
- The `ClientInterface` and protocol messages are spell-centric.

## Objective
Establish a unified `Actionable` interface and a centralized registry system to ensure consistency in how the bot interacts with the game world, regardless of whether it's casting a spell, using an ability, or using an item.

## Proposed Design

### 1. Unified `Actionable` Interface
Define a common interface that all game actions (Spell, Ability, Item) must implement. This will likely live in `internal/action`.

```go
type Actionable interface {
    GetName() string
    GetID() uint16
    GetActionType() ActionType // Spell, Ability, Item
    GetPriority() int
    GetTargetFlags() TargetFlags
}
```

### 2. Centralized Registries
Instead of selectors defining their own spells, create a central repository for all game data.
- `internal/registry/spells.go`: All spells in the game.
- `internal/registry/abilities.go`: All job abilities.
- `internal/registry/items.go`: Common items used by the bot.

### 3. Updated `ClientInterface`
Refactor the interface to be action-agnostic.

```go
type ClientInterface interface {
    SendAction(action Actionable, target string) error
    // ... existing methods like GetClientInfo
}
```

### 4. Protocol Evolution
Update `internal/protocol` to use a generic `ExecuteAction` message.

## Tasks
1. [x] Analyze existing structures for Spells, Abilities, and Items.
2. [ ] Define the `Actionable` interface in `internal/action`.
3. [ ] Design the centralized registry structure in `internal/registry`.
4. [ ] Map out the refactoring for `CastingEngine` to handle any `Actionable`.
5. [ ] Update `ClientInterface` definition in plan.
6. [ ] Define the updated JSON protocol for unified actions.
