# Queue Garbage Collection (GC) System

This document explains the logic and implementation of the Queue Garbage Collection (GC) system in PandaBot.

## Purpose

The Queue GC system ensures that the command queue remains relevant to the current game state. In a dynamic environment like FFXI, needs can change rapidly:
- Another player might heal a target before PandaBot does.
- A status effect might wear off naturally or be removed by another player.
- A player might die or leave the party, making a queued action impossible or unnecessary.

By removing these stale actions, the system saves MP and reduces "action lag."

## GC Logic

The GC process is triggered primarily when the server receives a `StatusUpdate` from a client. It evaluates each item in the `commandQueue` against the latest state in the `StatusMonitor`.

### 1. Healing Actions
- **Condition**: Action is a "Cure" or "Curaga" spell.
- **GC Rule**: Remove from queue if the target's current HP percentage is above the threshold that originally triggered the action (e.g., > 75% for a normal cure, or > 25% for a critical cure).
- **Optimization**: If a Curaga is queued and all its potential targets are now above the threshold, remove it.

### 2. Status Removal Actions
- **Condition**: Action is an "na" spell (e.g., Paralyna, Silena).
- **GC Rule**: Remove from queue if the specific status effect is no longer present on the target.

### 3. Invalid Targets
- **Condition**: Any action.
- **GC Rule**: Remove from queue if:
    - The target is no longer in the party.
    - The target is dead (unless the action is a Raise spell).
    - The target is out of range (if range information is reliable).

### 4. Self-Recovery (Silence)
- **Condition**: Action is "Echo Drop".
- **GC Rule**: Remove from queue if the player is no longer silenced.

## Implementation Details

The `validateQueuedActions` function in `internal/server/server.go` (or within the `CastingEngine`) will perform the following steps:
1. Acquire the `queueMutex` for the client.
2. Iterate through the `commandQueue`.
3. For each command, query the `StatusMonitor` for the target's current state.
4. Apply the GC rules listed above.
5. If a command fails the validation, remove it from the slice.
6. Log the removal for debugging purposes.

## Example Scenario

1. **T0**: Player A is at 10% HP. `Cure IV` is queued (Priority 80).
2. **T1**: Player B (another mage) casts `Cure V` on Player A.
3. **T2**: PandaBot receives a `StatusUpdate` showing Player A is now at 90% HP.
4. **T3**: Queue GC runs, sees `Cure IV` targeting Player A, checks HP (90% > threshold), and removes the command.
5. **T4**: PandaBot moves on to the next highest priority task (e.g., a buff).
