# Ashita v4 Lua Addon Guidelines

This project uses Ashita v4 exclusively. All Lua code MUST use the modern MemoryManager API.

## Required Patterns
- Party manager: `local party = AshitaCore:GetMemoryManager():GetParty();`
- Entity manager: `local entity = AshitaCore:GetMemoryManager():GetEntity();`
- Player HP/MP: `party:GetMemberHPPercent(0)`, `party:GetMemberMPPercent(0)`
- **Actual HP/MP values**: `party:GetMemberHP(i)`, `party:GetMemberMP(i)` (preferred for accurate calculations)
- Party member data (index 0-17): Check `party:GetMemberServerId(i) > 0` and same zone.
- Status effects: Use packet-based tracking (0x076 packets) - see Party Status Effects section below
- Distance (if needed): Use `party:GetMemberTargetIndex(i)` with `entity:GetDistance(target_index)`

## HP/MP Data Handling

### Required Pattern: Use Both Actual and Percentage Values
For accurate cure selection and spell calculations, always collect both percentage and actual HP/MP values:

```lua
-- Get both percentage and actual values for all party members
for i = 0, 17 do
    local server_id = party:GetMemberServerId(i)
    if server_id > 0 then
        local member_name = party:GetMemberName(i)
        if member_name and member_name ~= "" then
            -- Get both percentage and actual values
            local hp_percent = party:GetMemberHPPercent(i)
            local mp_percent = party:GetMemberMPPercent(i)
            local hp_actual = party:GetMemberHP(i)     -- Actual HP value
            local mp_actual = party:GetMemberMP(i)     -- Actual MP value
            
            -- Send both to Go server for accurate calculations
            table.insert(party_members, {
                name = member_name,
                hp_percent = hp_percent or 0,
                mp_percent = mp_percent or 0,
                hp_actual = hp_actual or 0,    -- Critical for cure selection
                mp_actual = mp_actual or 0,    -- Critical for MP management
                -- ... other fields
            })
        end
    end
end
```

### Why Both Values Are Important
- **Percentage values**: Reliable for basic health monitoring and thresholds
- **Actual values**: Essential for accurate cure selection, spell efficiency calculations, and precise healing
- **Cure selection**: Uses actual HP to calculate exact missing HP (e.g., 540/1200 HP = missing 660 HP)
- **MP management**: Uses actual MP for precise spell cost validation

### Required JSON Structure
When sending status updates to the Go server, include both value types:

```json
{
    "party_members": [
        {
            "name": "PlayerName",
            "hp_percent": 45,
            "mp_percent": 80,
            "hp_actual": 540,     // Actual current HP
            "mp_actual": 320,     // Actual current MP
            "status_effects": [...],
            // ... other fields
        }
    ]
}
```

## Chat Output and Printing

To print messages to the in-game chat log from Lua addons, use the standard Lua `print()` function. Messages printed this way appear in the chat window.

For colored text in chat messages, use FFXI's byte escape sequences directly in the string (no `chat` module or helper functions like `GetColoredString` exist in v4). The color change byte is `\31` (ASCII 31), followed by a color index byte (1-255). Always reset to default (`\31\1`) after colored sections to avoid bleeding colors.

### Required Pattern for Colored Chat Messages
- Use `string.char(31, color_index)` to start a color.
- Common color indices: 1 (default/white), 200 (cyan), 207 (yellow), 123 (green), 255 (red). Test with tools like BattleMod's `//bm colortest` for full list.
- Example: Colored initialization message

## Party Status Effects (Buffs/Debuffs)

Ashita v4's MemoryManager:GetParty() does **not** provide a `GetMemberStatusIcon(index, icon_index)` method (or similar per-icon access). Attempting to use it will cause "attempt to call method 'GetMemberStatusIcon' (a nil value)" errors.

### Required Pattern: Packet-Based Status Tracking
The standard and reliable way in v4 addons is to parse incoming **0x076 packets** (party member update packets) in the `packet_in` event. These packets contain a 32-bit bitmask (4 bytes) per member for active buffs/debuffs (bits 0-255 correspond to status IDs).

- Track status changes by maintaining a local table of current statuses per party member (by server ID or index).
- Update on zone-in, party changes, and every 0x076 packet.

### Recommended Implementation Guidelines
- In `packet_in` event:
    - If packet ID == 0x076:
        - Read the packet data starting after header (typically offset ~0x04 for member count, then per-member blocks).
        - Extract the 4-byte buff bitmask for each member.
        - Convert bitmask to list of active status IDs (bit operations: for bit 0 to 255, if (mask & (1 << bit)) ~= 0 then add bit + 1 or standard offset).
        - Common status ID mapping: Use known FFXI status IDs (e.g., 2=Sleep, 7=Paralyze; reference community resources or ResourceManager for names/icons).
- Maintain a global table like `party_statuses[member_index or server_id] = { status_ids }`.
- For your StatusUpdate message: Use this tracked table to populate `status[]` as integers.

### Alternative (Limited): Target-Only Memory Access
- For the **current target** only: Use `AshitaCore:GetMemoryManager():GetEntity():GetStatusIcons(target_index)` (returns up to 32 status IDs directly).
- Combine with `party:GetMemberTargetIndex(i)` to get statuses when a party member is targeted/sub-targeted.
- Not reliable for full party monitoring (requires constant targeting).

### Prohibited Patterns
- Never use `party:GetMemberStatusIcon(i, j)` or assume per-icon methods exist on the party object.
- Avoid outdated v3 assumptions about direct memory icon arrays per member.

### Additional Notes
- This packet method is used in popular v4 addons like statustimers (HealsCodes), status (official), and HXUI/tirem libraries.
- For icon display or names: Use `AshitaCore:GetResourceManager()` to get status details (e.g., icon paths, names).
- Reference repos: https://github.com/HealsCodes/statustimers, https://github.com/tirem/statuslib

## Prohibited Patterns (Ashita v3 – NEVER use these)
- Any global tables like `entities`, `party`, or `inventory`
- Chained calls like `AshitaCore:GetDataManager():GetParty()` or `GetEntity()`
- `entities:GetEntity(index)` or similar v3 helpers

## Additional Rules
- Always prefer real v4 community examples (e.g., tirem/XivParty, ThornyFFXI addons on GitHub).
- Reference: Official features doc at https://docs.ashitaxi.com/features/
- When generating status monitoring or party data code, strictly follow the patterns above.