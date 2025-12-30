# PandaBot Queue System Documentation

This document describes the design and implementation of the queue systems in PandaBot, covering both the Go server and the Lua addon.

## Overview

PandaBot utilizes several queuing mechanisms to ensure reliable communication and prioritized execution of commands:

1.  **Server-Side Command Queue**: Manages commands sent from the server to each client.
2.  **Client-Side Message Queue**: Buffers outgoing messages from the addon to the server during network interruptions.

---

## 1. Server-Side Command Queue (Go)

The server maintains the master command queue for each connected client (one per party member). This ensures that actions for different players are managed independently and that the Lua addon remains a thin execution layer.

### Data Structures
- **`QueuedCommand`**: Represents a single command with its state, priority, and metadata.
- **`Client.commandQueue`**: A slice of `*QueuedCommand` pointers.
- **`Client.currentCommand`**: Tracks the command currently being executed by the client.

### Command Lifecycle
1.  **Queueing**: When a component (like `triggerService`) needs to send a command, it calls `queueCommandForClient`.
    - Commands are assigned a unique ID (`cmd_<timestamp>`).
    - Commands are inserted into the master queue in **priority order** (higher priority values first).
2.  **Processing**: `processCommandQueue` is triggered whenever a new command is queued or a previous command completes.
    - If `currentCommand` is not nil, it checks for a **timeout** (default 30 seconds).
    - If no command is in progress, it pops the highest priority command from the queue and sends it to the client.
3.  **Execution**: The client receives the command and executes it immediately. The client does NOT maintain its own queue.
4.  **Completion/Failure**: The server waits for the client to report back.
    - `handleSpellComplete` / `handleJSONSpellComplete` marks the command as `CommandCompleted`.
    - `handleSpellFailed` / `handleJSONSpellFailed` marks it as `CommandFailed`.
    - Once marked, `currentCommand` is cleared, and the next command in the queue is automatically processed.

---

## 2. Client-Side Execution (Lua)

The Lua addon acts as a stateless executor for commands provided by the server. It does not buffer or re-prioritize commands locally.

### Key Components
- **`handle_execute_command(command_data)`**: Receives `TypeExecuteCommand` messages.
    - Immediately attempts to execute the command using `ashita_chat:QueueCommand`.
    - Reports success or failure back to the server.
    - If the client is already executing a command (e.g. waiting for a previous spell to finish), the server is responsible for not sending another until completion is reported.

---

## 3. Client-Side Message Queue (Lua)

To handle network instability, the addon buffers messages intended for the server.

### Functionality
- **`message_queue`**: Stores outgoing JSON messages.
- **`send(msg)`**: 
    - If connected, attempts to send the message immediately.
    - If disconnected or sending fails, the message is `table.insert`ed into `message_queue`.
    - Enforces `max_message_queue_size` (removes oldest if full).
- **`send_queued_messages()`**: 
    - Upon reconnection, it attempts to drain the queue by sending all buffered messages in order.

---

## Priority Hierarchy

The system uses a standardized priority hierarchy to ensure critical actions are always performed first. Higher numerical values indicate higher priority.

| Priority Level | Numerical Value | Action Type | Examples |
| :--- | :--- | :--- | :--- |
| **Highest** | 100 | Self-Recovery | Echo Drop (Silence), Item Usage |
| **Higher** | 80 | Critical Healing | Cure spells when HP <= 20% |
| **High** | 60 | Vital Status Removal | Stona, Paralyna, Silena |
| **Normal** | 40 | Routine Healing | Non-critical cures (HP > 20%) |
| **Lowest** | 20 | Utility | Buffs, Shell, Protect, etc. |

### Queue Item Structure

Every item in the queue (represented as `QueuedCommand` on the server and `command_entry` in Lua) contains the following essential information:

- **Action**: The actual command to execute (e.g., `/item "Echo Drop" <me>`, `/ma "Cure IV" <t>`).
- **Target**: The recipient of the action (e.g., "Player1", "<me>", "<t>").
- **Priority**: The numerical priority level (1-100).
- **ID**: A unique identifier for tracking the lifecycle of the action.
- **Timestamp**: When the action was first queued.

---

## Configuration Summary

The following settings in `cmd\addon\pandabot.lua` and `internal\server\server.go` control the queue behavior:

| Setting | Default Value | Description |
| :--- | :--- | :--- |
| `max_message_queue_size` | 100 | Max outgoing messages buffered in Lua |
| `max_command_queue_size` | 100 | Max incoming commands buffered in Lua |
| `command_timeout` | 30000ms | Default timeout for command execution |
| `StatusUpdateInterval` | 5s | Frequency of status updates (which may trigger new commands) |

---

## Technical Details

### Priority Handling
Both the Go server and Lua addon use numerical priority values. In both cases, **higher numbers indicate higher priority**.
- Server: `if priority > existingCmd.Priority { ... insert ... }`
- Lua: `table.sort(command_queue, function(a, b) return a.priority > b.priority end)`

### Error Handling
If a command fails to execute in Lua (e.g., `pcall` on `QueueCommand` fails), a `TypeSpellFailed` message is sent back to the server immediately, allowing it to move on to the next command in its queue.
