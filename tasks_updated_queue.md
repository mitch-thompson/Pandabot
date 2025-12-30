# Implementation Tasks: Updated Queue System

This document outlines the tasks required to implement the centralized server-side queue system as defined in the updated `requirements.md` and `design.md`.

## 1. Server-Side Implementation (Go)

### 1.1 Data Structure Updates
- [x] Update `QueuedCommand` struct in `internal/server/server.go` to ensure it fully supports the 1-100 priority scale.
- [x] Ensure `ExecuteCommand` in `internal/protocol/types.go` matches the required metadata (Action, Target, Priority, ID, Timestamp).
- [x] Implement a constant or configuration for `MaxCommandQueueSize = 100` per client.

### 1.2 Centralized Casting System Integration
- [x] Refactor `CastingEngine` to be the sole entry point for adding commands to the queue. (Refactored RequestCast to use queueCommandForClient)
- [x] Implement priority-based insertion in the `Client.commandQueue` (higher numbers first).
- [x] Implement "Self-Recovery Preemption" (Priority 100):
    - [x] When an Echo Drop (Priority 100) is queued, it should be placed at the very front.
    - [x] If Requirement 10.4 (interrupt current queue) is strictly interpreted as clearing the queue, implement that logic.

### 1.3 Command Lifecycle Management
- [x] Update `processCommandQueue` to enforce serial execution:
    - [x] Verify `currentCommand` is NIL before sending the next one.
    - [x] Implement robust timeout handling (default 30s) to clear `currentCommand` if the client fails to report back.
- [x] Ensure `handleJSONSpellComplete` and `handleJSONSpellFailed` correctly clear `currentCommand` and trigger `processCommandQueue`.
- [x] Implement Queue Garbage Collection (GC):
    - [x] Develop a `validateQueuedActions` function that compares the `commandQueue` against the latest `statusMonitor` state.
    - [x] Remove heal actions if the target's HP > threshold.
    - [x] Remove status removal actions if the debuff is no longer present on the target.
    - [x] Remove any actions targeting a player who is no longer in the party or is marked as dead.
    - [x] Trigger GC upon receiving a `StatusUpdate` from any client.

## 2. Client-Side Implementation (Lua)

### 2.1 Stateless Execution
- [x] Verify `pandabot.lua` has no local queue or buffering logic.
- [x] Ensure `handle_execute_command` processes the command immediately and triggers game execution.
- [x] Verify that every execution path (success, failure, pcall error) sends a response back to the server.

### 2.2 Completion Reporting
- [x] Improve spell completion detection in Lua (moving beyond the current 3-second simulation if possible).
- [x] Ensure `TypeSpellComplete` and `TypeSpellFailed` messages carry the correct `id` (and added server-side fallbacks).

## 3. Testing and Verification

### 3.1 Unit Testing
- [x] Update `internal/server/command_queue_test.go` to test:
    - [x] Insertion order with priorities 20, 40, 60, 80, 100.
    - [x] Queue limit (100 items) and eviction policy.
    - [x] Serial execution (waiting for completion).

### 3.2 Property-Based Testing
- [x] Implement Property 3 (Command queue management) in `design.md`:
    - [x] Generate random commands with various priorities.
    - [x] Verify they are dispatched in the correct order.
- [x] Implement Property 11 (Action prioritization ranking).

### 3.3 Integration Testing
- [x] Simulate a "Silence" event and verify Echo Drop (Priority 100) pre-empts or clears lower priority heals/buffs.
- [x] Verify system recovery after a simulated network timeout.
