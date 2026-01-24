### Buff Selection Logic (Protect and Shell)

The PandaBot's buff selection system (implemented in `internal/buffSelector/buffSelector.go`) is designed to intelligently choose between single-target spells (e.g., Protect) and area-of-effect (AoE) spells (e.g., Protectra) based on the current party context.

#### Core Selection Criteria

The selector follows these rules when determining the optimal spell:

1.  **Target Context**:
    *   **AoE Spells (Protectra/Shellra)**: These are only considered if:
        *   The target is the caster (`targetIsSelf` is true).
        *   The party size is 3 or more members.
        *   It is the first attempt (not a fallback).
    *   **Single-Target Spells (Protect/Shell)**: These are used if:
        *   The target is someone other than the caster.
        *   The party size is less than 3.
        *   An AoE spell was preferred but none were available (fallback mechanism).

2.  **Job Level Requirements**:
    *   The system checks the caster's current job levels against the `LevelReq` defined in the spell registry.
    *   A spell is only eligible if the caster meets the minimum level requirement for any of their active jobs (typically the main job).

3.  **MP Availability**:
    *   The caster must have sufficient MP to cast the spell.
    *   The `CastingEngine` subtracts a configured `MPReservation` (defaulting to 0, but adjustable) from the available MP before checking requirements.

4.  **Priority-Based Selection**:
    *   Among all eligible spells (those that match the target type, level, and MP requirements), the system selects the one with the highest `Priority` (which corresponds to the spell's tier, e.g., Protect V has a higher priority than Protect IV).

#### Fallback Mechanism

If the system prefers an AoE spell (due to party size and self-targeting) but no suitable AoE spell is found (e.g., due to MP or level constraints), it will automatically fall back to searching for the best available single-target spell.

#### Trigger Integration

When a "protect" or "shell" trigger is detected in chat:
*   The `TriggerService` routes the event to the `CastingEngine`.
*   The `CastingEngine` evaluates the `CastContext` (party size, caster MP, caster name vs. target).
*   The `BuffSelector` returns the specific spell name (e.g., "Protectra V").
*   The command is then queued for execution (e.g., `/ma "Protectra V" <me>`).
