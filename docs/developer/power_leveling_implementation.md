### Power Leveling (PL) Mode Implementation

This document describes the technical implementation of Power Leveling (PL) mode, specifically how Curaga spells, 'na spells, and Erase are disabled to manage enmity and focus resources.

#### Core State: `isPowerleveling`

The `isPowerleveling` state is a boolean flag that indicates whether the bot is currently in a mode where it should avoid Area of Effect (AoE) heals.

#### 1. Activation and State Management
PL mode is toggled via in-game chat triggers (tells or party chat) handled in `internal/server/server.go`.

- **Detection**: The `Server.handleJSONChatMessage` method monitors for "power level" and "stop pl" strings. It allows activation from tells (modes 3, 4, 13, 14) and party chat (mode 12).
- **Client State**: Each connected client has `PLSource` and `PLTarget` fields.
- **Dynamic Flag**: The `isPowerleveling` flag is derived dynamically during action selection:
  ```go
  isPL := (plTarget == playerName && plSource != "")
  ```

#### 2. State Propagation
The flag is passed through several layers to ensure all casting methods respect the PL restriction.

##### Automatic Action Loop
In `internal/autoActionService/autoActionService.go`, `DecideNextAction` determines if PL mode is active for the current client and passes it to the casting engine:
- `castingEngine.SelectOptimalCure(context)` where `context.IsPowerleveling = isPL`.
- `castingEngine.SelectOptimalNaAction(context)` where `context.IsPowerleveling = isPL`.

##### Trigger-Based Actions (Tells)
When a player sends a tell (e.g., "heal"), the flow is:
1.  `internal/server/server.go`: Receives tell, calls `RouteTriggerEvents`.
2.  `internal/triggerService/triggerService.go`: Calculates `isPL` and calls `ts.castingSystem.ProcessTriggerEvent`.
3.  `internal/casting/server_adapter.go`: `ProcessTriggerEvent` accepts `isPowerleveling` and passes it to the `TriggerProcessor`.
4.  `internal/casting/trigger_processor.go`: `ProcessTriggerEvent` creates a `CastContext` with `IsPowerleveling` set.

##### Helper/Convenience Methods
The `internal/casting/convenience.go` package provides helper methods like `CastCure`, `CastBuffs`, and `CastPartyCures`. All these methods have been updated to accept an `isPowerleveling bool` parameter, ensuring that manual or programmatic calls also respect the mode.

#### 3. Enforcement Logic
The actual blocking of Curaga, 'na spells, and Erase happens in the `cureSelector` and `CastingEngine`.

- **`internal/cureSelector/cureSelector.go`**:
    - `getAvailableCuragaOptions`: Returns `nil` immediately if `isPowerleveling` is true.
    - `SelectCuragaForMultipleTargets`: Returns an error if `isPowerleveling` is true.
    - `ShouldUseCuraga`: Returns `false` if `isPowerleveling` is true.
    - `SelectOptimalCure`: Skips the Curaga evaluation block if `isPowerleveling` is true.

- **`internal/casting/casting.go`**:
    - The `CastContext` struct holds the `IsPowerleveling` flag.
    - `selectOptimalCure` and `selectOptimalBuffs` pass this flag down to their respective selectors.
    - `selectOptimalNaSpell`: Returns an error immediately if `IsPowerleveling` is true, effectively disabling all 'na spells (Poisona, Paralyna, etc.) and Erase.

- **`internal/autoActionService/autoActionService.go`**:
    - `DecideNextAction`: Explicitly skips status effect removal loops if `isPowerleveling` is true to avoid unnecessary processing and log spam.

#### 4. Summary of Key Methods

| Method | File | Purpose |
| :--- | :--- | :--- |
| `DecideNextAction` | `autoActionService.go` | Entry point for auto-actions, calculates PL state. |
| `RouteTriggerEvents` | `triggerService.go` | Entry point for tells, calculates PL state. |
| `ProcessTriggerEvent` | `trigger_processor.go` | Converts triggers to `CastRequest` with PL context. |
| `SelectOptimalCure` | `cureSelector.go` | Selects best cure, excludes Curaga if PL is active. |
| `getAvailableCuragaOptions`| `cureSelector.go` | Central gatekeeper for Curaga spell availability. |
| `CastPartyCures` | `convenience.go` | Orchestrates healing for multiple targets, respects PL. |
