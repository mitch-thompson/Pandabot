# Implementation Tasks: Decision Tree Revision

This document outlines the tasks required to revise the queue system to use a decision tree approach with desired buff monitoring, as defined in the updated `requirements.md` and discussion.

## 1. Status Monitor Updates (statusMonitor.go)

- [ ] Update `DesiredBuff` struct to include `Priority int` and `Expiry time.Time`.
- [ ] Add `ClearDesiredBuffs()` method to clear all desired buffs.
- [ ] Add `ClearDesiredBuff(target string)` to clear buffs for a specific target.
- [ ] Add `ClearDesiredBuffBySpell(spellName string)` to clear a specific buff type.
- [ ] Integrate zone change detection to clear desired buffs based on configuration (e.g., via `zone.go` events).
- [ ] Update `RegisterDesiredBuff` to accept priority and optional expiry.
- [ ] Enhance `GetMissingDesiredBuffs()` to return sorted by priority descending.
- _Requirements: 3.7, 3.8, 3.9, 4.6_

## 2. Text Parser and Trigger Service Updates (textParser.go, triggerService.go)

- [ ] Add parsing for "panda clear" trigger in `ParseMessage`, supporting variants: plain (clear all), with spell name (clear specific), with player name (clear for target).
- [ ] On buff triggers (e.g., "haste", "firebuffs", "whmprep"), register `DesiredBuff`s with appropriate priorities (e.g., Reraise:90, Light Arts:80) instead of immediate casting.
- [ ] For self-buffs, use `"<me>"` as target.
- [ ] Add rate-limiting or cooldowns to prevent trigger spam.
- _Requirements: 2.6, 4.6_

## 3. Auto Action Service Implementation (autoActionService.go)

- [ ] Implement `DecideNextAction(client *Client) (*protocol.ExecuteCommand, error)` with the full decision tree:
    - Self-checks (silence/echo drops, low MP).
    - Critical heals (Curaga evaluation via cureSelector).
    - High debuffs (via naSelector).
    - Mid heals.
    - Missing desired buffs (sorted by priority).
    - Low debuffs.
    - Return nil for idle.
- [ ] Integrate with selectors (cureSelector, naSelector, buffSelector) for spell resolution.
- [ ] Use `prioritizer.go` for dynamic priority adjustments.
- [ ] Add MP and readiness checks before returning commands.
- _Requirements: 1.6, 4.1-4.8, 10.1-10.5_

## 4. Server and Casting Integration (server.go, casting.go)

- [ ] In `handleReadyForAction`, call `autoActionService.DecideNextAction` to get command; if non-nil, send via `SendSpellCommand`; else, idle.
- [ ] Update `handleSpellComplete` to update `StatusMonitor` (e.g., assume buff applied, clear desired if matched).
- [ ] Remove or deprecate queue-related code in `Client.commandQueue`, shifting to tree-based decisions.
- [ ] Enhance `CastingEngine` to handle any remaining sequences if needed (e.g., mini-chains for complex buffs).
- [ ] Add logging for tree decisions and buff clears.
- _Requirements: 1.1-1.8, 4.1_

## 5. Configuration Updates (config.go, config.toml)

- [ ] Add configurable options for buff expiries (default indefinite, or duration-based).
- [ ] Add flag for auto-clear on zone change (default: true for non-persistent buffs like elementals, false for Reraise).
- [ ] Add thresholds for heals/debuffs in decision tree (e.g., critical HP <30%).
- [ ] Ensure reloadable via fsnotify.
- _Requirements: 3.9_

## 6. Protocol and Lua Addon Adjustments (types.go, pandabot.lua)

- [ ] If needed, add messages for buff clear acknowledgments or zone events.
- [ ] Ensure Lua sends zone changes in StatusUpdate for clearing.
- [ ] No major changes expected, but verify ready-for-action flow.
- _Requirements: 7.1-7.5, 11.1-11.8_

## 7. Testing

- [ ] Unit tests for decision tree branches (e.g., silence preempts buffs, critical heals over low debuffs).
- [ ] Property-based tests for buff prioritization and sequencing (e.g., simulate missing buffs, verify order).
- [ ] Integration tests: Trigger "firebuffs", verify tree casts in priority order across multiple ready pings.
- [ ] Test "panda clear" variants and zone clears.
- [ ] Update existing tests (e.g., buff_monitoring_test.go) for new structs/methods.
- _Requirements: All_

## 8. Documentation Updates

- [ ] Update `design.md` with decision tree diagram and buff monitoring flow.
- [ ] Revise `QUEUE_SYSTEM.md` to describe tree-based system (or rename to DECISION_TREE.md).
- [ ] Add user guide for "panda clear" in README.md.
- [ ] Final checkpoint: Ensure all tests pass.