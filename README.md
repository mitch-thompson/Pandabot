# PandaBot

PandaBot is an automated gameplay assistant for Final Fantasy XI, designed to work with **Ashita v4**. It consists of a high-performance Go-based server that manages bot logic and a Lua-based addon that interfaces with the game client.

### Features

- **Automated Healing**: Intelligent cure and curaga selection for party members.
- **Status Removal**: Automatic removal of debuffs (Poison, Paralysis, etc.) using appropriate spells.
- **Buff Maintenance**: Keeps essential buffs active on yourself and your party.
- **Power Leveling Mode**: Specialized logic for power-leveling other characters.
- **Blue Magic Support**: Specialized healing logic for Blue Mages.
- **Trigger Actions**: Custom reactions to in-game text or events.
- **GUI Dashboard**: Real-time monitoring of party status and bot activity.

### Installation

#### 1. Server (Go Application)

You need to have [Go 1.25+](https://go.dev/) installed to build the server.

```bash
go build -o pandabot.exe ./cmd/pandabot
```

Alternatively, you can find pre-built binaries in the GitHub Releases if available.

#### 2. Addon (Ashita v4)

1. Locate your Ashita v4 installation directory.
2. Copy the `cmd/addon` folder into your `addons` directory and rename it to `pandabot`.
   - Path should look like: `Ashita/addons/pandabot/pandabot.lua`

### Usage

1. **Launch the Server**: Run `pandabot.exe`. A GUI window will appear showing the party status.
2. **Launch FFXI via Ashita v4**.
3. **Load the Addon**: In the game chat, type:
   ```
   /addon load pandabot
   ```
4. The addon will connect to the server (default: `127.0.0.1:31337`).

### Trigger Actions

PandaBot responds to specific chat triggers from party members or via direct tells.

| Category | Triggers | Description |
|----------|----------|-------------|
| **Control** | `panda clear` | Clears all tracked buffs for everyone. |
| | `panda clear <name>` | Clears tracked buffs for a specific player. |
| | `panda clear <spell>` | Clears a specific tracked buff (e.g., `panda clear haste`). |
| | `panda` | Clears the current casting queue and all tracked buffs. |
| **System** | `power level` | Enters Power Leveling mode (sent via tell to the bot). |
| | `stop pl` | Disables Power Leveling mode. |
| | `disable cures` | Suspends all automatic healing and status removal. |
| | `enable cures` | Resumes automatic healing and status removal. |
| **Healing** | `heal`, `cure`, `help` | Casts the optimal Cure based on missing HP. |
| **Status** | `erase`, `paralyzed`, `silenced`, etc. | Removes the specified status effect. |
| **Buffs** | `haste`, `regen`, `protect`, `shell` | Applies the requested buff. |
| | `solace`, `misery`, `lightarts`, `darkarts` | Switches stance or uses job-specific buffs. |

For a complete list of triggers, see [Trigger Actions Documentation](docs/trigger_actions.md).

### Documentation

Detailed documentation for specific features can be found in the `docs/` folder:

- [Cure Selection](docs/cure_selection.md)
- [Buff Selection](docs/buff_selection.md)
- [Power Leveling](docs/power_leveling.md)
- [Trigger Actions](docs/trigger_actions.md)
- [Blue Magic Healing](docs/blue_magic_healing.md)
- [Plugin Build Guide](docs/plugin_build.md)

### Development

For information on the internal architecture and contributing, see [flow.md](flow.md).

### License

This project is licensed under the MIT License - see the LICENSE file for details.
