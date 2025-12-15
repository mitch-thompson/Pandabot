# Requirements Document

## Introduction

The Automated Gameplay Assistant is a system consisting of a Lua addon for Ashita v4 and a Go server that provides intelligent spell casting, status monitoring, and text parsing capabilities. The system enables automated responses to party communications, proactive healing and status management, and prioritized action execution based on real-time game state analysis. The system is designed to work specifically with Ashita v4's addon architecture, memory pointers, and command system.

## Glossary

- **Lua_Addon**: The client-side Ashita v4 addon that interfaces with the game using Ashita's addon framework and communicates with the Go server
- **Go_Server**: The server-side application that processes game data, makes decisions, and sends commands back to the addon
- **Action_Command**: A structured command sent from server to addon (e.g., "/ma \"Cure IV\" player_name")
- **Status_Message**: Periodic data sent from addon to server containing party member health, MP, and status information
- **Text_Parser**: Server component that analyzes incoming chat messages for trigger words and creates generic trigger events
- **Casting_System**: Centralized server component that handles all spell selection, target resolution, priority management, and casting coordination
- **Spell_Prioritizer**: Component within the Casting_System that determines optimal spell selection and casting order based on current needs
- **Party_Member**: Any player character in the current party whose status is monitored
- **Memory_Pointers**: Ashita v4 memory pointer system for accessing game data directly from memory
- **Command_System**: Ashita v4 mechanism for executing game commands through the addon framework

## Requirements

### Requirement 1

**User Story:** As a player, I want the Lua plugin to execute commands sent from the Go server, so that automated actions can be performed in the game.

#### Acceptance Criteria

1. WHEN the Go_Server sends an Action_Command to the Lua_Addon, THEN the Lua_Addon SHALL execute the command in the game client using Ashita v4's command system
2. WHEN an Action_Command contains spell casting instructions, THEN the Lua_Addon SHALL parse the spell name and target correctly and execute via Ashita v4's command execution
3. WHEN an Action_Command is malformed or invalid, THEN the Lua_Addon SHALL log the error using Ashita v4's print functions and continue normal operation
4. WHEN the Lua_Addon receives multiple Action_Commands, THEN the Lua_Addon SHALL execute them based on priority, with critical actions preempting lower priority actions
5. WHEN an Action_Command execution fails, THEN the Lua_Addon SHALL report the failure status back to the Go_Server
6. WHEN multiple spells need to be cast in sequence, THEN the Go_Server SHALL queue commands server-side and only send the next command after receiving completion notification from the Lua_Addon for the previous command
7. WHEN a spell finishes casting, THEN the Lua_Addon SHALL notify the Go_Server of spell completion to trigger the next queued command

### Requirement 2

**User Story:** As a player, I want the system to monitor and respond to specific words in tells and party messages, so that appropriate healing and buff spells are automatically cast.

#### Acceptance Criteria

1. WHEN a Party_Member sends a tell (mode 3) or party message (mode 4) containing trigger words, THEN the Lua_Addon SHALL capture only these specific message types using Ashita v4's text event callbacks and forward them to the Go_Server for processing
2. WHEN the Text_Parser identifies trigger words in a message, THEN the Text_Parser SHALL create a generic trigger event with the trigger type and sender information without performing any spell selection or target resolution
3. WHEN a trigger event is created, THEN the Go_Server SHALL pass the trigger event to the Casting_System for all spell selection, target resolution, and command generation
4. WHEN the Casting_System receives a trigger event, THEN the Casting_System SHALL determine appropriate spells, resolve correct targets based on spell requirements, and generate prioritized casting requests
5. WHEN a trigger word is detected from an unknown player, THEN the Go_Server SHALL ignore the request and log the event

### Requirement 3

**User Story:** As a player, I want the system to continuously monitor party status using accurate Ashita v4 data, so that proactive healing and status management can be performed.

#### Acceptance Criteria

1. WHEN the game is running, THEN the Lua_Addon SHALL use Ashita v4's render event callbacks to send Status_Messages to the Go_Server at regular intervals
2. WHEN a Status_Message is sent, THEN the Lua_Addon SHALL use Ashita v4's MemoryManager to read both actual HP/MP values (`party:GetMemberHP(i)`, `party:GetMemberMP(i)`) and percentage values (`party:GetMemberHPPercent(i)`, `party:GetMemberMPPercent(i)`) for all Party_Members
3. WHEN a Party_Member's health drops below critical thresholds, THEN the Go_Server SHALL determine appropriate cure spell level and target using actual HP values for precise missing HP calculations
4. WHEN a Party_Member has negative status effects, THEN the Go_Server SHALL determine appropriate "na" spell and target
5. WHEN Status_Messages are not received within expected intervals, THEN the Go_Server SHALL log connection issues and attempt reconnection
6. WHEN actual HP/MP values are available from Ashita v4, THEN the Go_Server SHALL prioritize these over percentage-based estimates for all healing calculations

### Requirement 4

**User Story:** As a player, I want the server to intelligently prioritize spell casting, so that the most critical actions are performed first.

#### Acceptance Criteria

1. WHEN multiple healing needs are detected simultaneously, THEN the Spell_Prioritizer SHALL rank actions by urgency and importance
2. WHEN a Party_Member has critically low health, THEN the Spell_Prioritizer SHALL prioritize cure spells over buff spells
3. WHEN multiple Party_Members need healing, THEN the Spell_Prioritizer SHALL prioritize based on health percentage and role importance
4. WHEN status removal spells are needed, THEN the Spell_Prioritizer SHALL prioritize life-threatening conditions over minor debuffs
5. WHEN spell casting resources are limited, THEN the Spell_Prioritizer SHALL optimize MP usage and casting efficiency

### Requirement 5

**User Story:** As a player, I want the system to determine appropriate cure spell levels using accurate HP data, so that healing is efficient and effective.

#### Acceptance Criteria

1. WHEN a Party_Member needs healing, THEN the Go_Server SHALL calculate the optimal cure spell level based on actual missing HP values from Ashita v4's `party:GetMemberHP(i)` method when available
2. WHEN actual HP values are not available, THEN the Go_Server SHALL fall back to percentage-based calculations with improved job-level HP estimation (15-20 HP per level with job-specific multipliers)
3. WHEN a Party_Member has minor damage (less than 200 missing HP), THEN the Go_Server SHALL select lower-tier cure spells to conserve MP while ensuring adequate healing coverage
4. WHEN a Party_Member has critical damage (more than 400 missing HP), THEN the Go_Server SHALL prioritize healing effectiveness over MP efficiency and select higher-tier cure spells for maximum healing
5. WHEN multiple cure options are available, THEN the Go_Server SHALL use enhanced scoring that balances MP efficiency with healing appropriateness, giving 1.5x bonus for 70-120% coverage and penalties for insufficient (<50%) or excessive (>200%) healing
6. WHEN cure spell selection is made, THEN the Go_Server SHALL verify the caster has sufficient MP before sending the Action_Command
7. WHEN three or more Party_Members need healing simultaneously, THEN the Go_Server SHALL select appropriate curaga spells instead of individual cure spells for improved efficiency
8. WHEN the Lua_Addon sends status updates, THEN it SHALL include both `hp_actual`/`mp_actual` values from Ashita v4's memory pointers AND percentage values for comprehensive healing calculations

### Requirement 6

**User Story:** As a player, I want the system to determine appropriate status removal spells, so that negative effects are efficiently removed.

#### Acceptance Criteria

1. WHEN a Party_Member has negative status effects, THEN the Go_Server SHALL identify the specific "na" spell required
2. WHEN multiple status effects are present, THEN the Go_Server SHALL prioritize removal based on effect severity
3. WHEN a status effect requires a specific "na" spell, THEN the Go_Server SHALL send the correct Action_Command with proper targeting
4. WHEN status removal spells are unavailable or insufficient MP exists, THEN the Go_Server SHALL queue the action for later execution
5. WHEN status effects are successfully removed, THEN the Go_Server SHALL update its internal tracking of Party_Member conditions

### Requirement 7

**User Story:** As a system administrator, I want reliable communication between the Lua plugin and Go server, so that the system operates consistently.

#### Acceptance Criteria

1. WHEN the Lua_Addon loads via Ashita v4's addon system, THEN the Lua_Addon SHALL establish a connection to the Go_Server
2. WHEN network connectivity is lost, THEN both components SHALL attempt automatic reconnection with exponential backoff
3. WHEN messages are sent between components, THEN the system SHALL use a simple text-based protocol for reliable data transmission
4. WHEN the Go_Server is unavailable, THEN the Lua_Addon SHALL continue basic operation using Ashita v4's addon framework and queue messages for later transmission
5. WHEN connection is restored after an outage, THEN both components SHALL synchronize state and resume normal operation

### Requirement 8

**User Story:** As a developer, I want all spell casting logic centralized in a single system, so that target resolution, spell selection, and casting coordination are consistent and maintainable.

#### Acceptance Criteria

1. WHEN any component needs to cast a spell, THEN it SHALL use the Casting_System as the single point of entry for all casting operations
2. WHEN the Casting_System receives a casting request, THEN it SHALL perform all target resolution based on spell metadata and targeting requirements without relying on external components
3. WHEN a spell has self-targeting requirements (like Bar spells), THEN the Casting_System SHALL automatically resolve the target to the caster regardless of the original request target
4. WHEN a spell has party-member targeting requirements (like Cure spells), THEN the Casting_System SHALL validate and resolve the target based on party membership and spell constraints
5. WHEN multiple spells need to be cast in sequence, THEN the Casting_System SHALL manage the entire sequence including priority ordering, timing, and target resolution for each individual spell
6. WHEN spell selection is required (like optimal cure level or buff sequence), THEN the Casting_System SHALL use its integrated selectors without external components making spell decisions
7. WHEN casting requests are made, THEN no other component SHALL perform target resolution, spell selection, or casting logic outside of the Casting_System
8. WHEN the Text_Parser detects triggers, THEN it SHALL only identify trigger types and pass raw trigger events to the Casting_System without any spell or target information
9. WHEN the Server processes triggers, THEN it SHALL only route trigger events to the Casting_System without performing any casting-related logic

### Requirement 9

**User Story:** As a developer, I want the cure selection system to use accurate HP data and intelligent scoring, so that healing decisions are both effective and efficient.

#### Acceptance Criteria

1. WHEN calculating missing HP for cure selection, THEN the system SHALL prioritize actual HP values from Ashita v4's `party:GetMemberHP(i)` method over percentage-based estimates
2. WHEN actual HP values are unavailable, THEN the system SHALL use enhanced job-level HP estimation with 15-20 HP per level and job-specific multipliers (melee jobs 1.2x, mage jobs 0.9x)
3. WHEN scoring cure options, THEN the system SHALL use healing appropriateness scoring that gives 1.5x bonus for 70-120% coverage, 0.3x penalty for <50% coverage, and 0.6x penalty for >200% coverage
4. WHEN prioritizing efficiency vs effectiveness, THEN the system SHALL balance MP efficiency with healing appropriateness in efficiency mode, and prioritize healing effectiveness and speed in emergency mode
5. WHEN multiple cure spells can heal the required amount, THEN the system SHALL select the spell that provides the best balance of healing coverage and resource efficiency
6. WHEN the cure selector receives both actual and percentage HP/MP values, THEN it SHALL use actual values for precise calculations and fall back to percentages only when actual values are unavailable

### Requirement 10

**User Story:** As a developer, I want the system to be fully compatible with Ashita v4, so that it works reliably with the current Ashita architecture.

#### Acceptance Criteria

1. WHEN the Lua_Addon loads, THEN it SHALL use Ashita v4's addon framework for initialization and event handling
2. WHEN accessing game data, THEN the Lua_Addon SHALL use Ashita v4's MemoryManager system (`AshitaCore:GetMemoryManager():GetParty()`) to read party member information directly from game memory
3. WHEN collecting HP/MP data, THEN the Lua_Addon SHALL use both actual value methods (`party:GetMemberHP(i)`, `party:GetMemberMP(i)`) and percentage methods (`party:GetMemberHPPercent(i)`, `party:GetMemberMPPercent(i)`) for comprehensive data collection
4. WHEN executing game commands, THEN the Lua_Addon SHALL use Ashita v4's command execution system to send spells and actions
5. WHEN monitoring chat messages, THEN the Lua_Addon SHALL use Ashita v4's text event callbacks to capture tells and party messages
6. WHEN the addon needs to access player status, THEN it SHALL use Ashita v4's memory pointers to retrieve HP, MP, and status effect data from game structures
7. WHEN handling addon lifecycle, THEN the Lua_Addon SHALL properly implement Ashita v4's load, unload, and command event handlers
8. WHEN communicating with the server, THEN the Lua_Addon SHALL use JSON protocol over TCP sockets and include both actual and percentage HP/MP values in status updates