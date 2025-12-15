# Ashita v4 Compatibility Guide

## Changes Made

The addon has been refactored to be fully compatible with Ashita v4. Here are the key changes:

### 1. API Manager Access
- **Before (v5 attempt)**: Fallback methods and compatibility checks
- **After (v4 compatible)**: Direct API access at initialization
```lua
-- v4 compatible - direct initialization
local chat = AshitaCore:GetChatManager()
local memory = AshitaCore:GetMemoryManager()
local entities = AshitaCore:GetEntityManager()
local party = AshitaCore:GetPartyManager()
```

### 2. Status Effects Access
- **Before (v5 attempt)**: Multiple fallback methods
- **After (v4 compatible)**: Direct StatusServer access
```lua
-- v4 compatible - direct access
if entity and entity.StatusServer then
    for j = 0, 31 do
        local status_id = entity.StatusServer[j]
        if status_id and status_id > 0 then
            table.insert(status_effects, status_id)
        end
    end
end
```

### 3. Command Execution
- **Before (v5 attempt)**: Multiple execution methods with fallbacks
- **After (v4 compatible)**: Direct QueueCommand method
```lua
-- v4 compatible - direct method call
chat:QueueCommand(1, command)
```

### 4. Event Registration
- **Before (v5 attempt)**: Complex error handling and fallback methods
- **After (v4 compatible)**: Direct event registration
```lua
-- v4 compatible - direct registration
ashita.events.register('text_in', 'pandabot_chat', handle_chat_message)
```

### 5. Module Loading
- **Before (v5 attempt)**: Protected socket loading with error handling
- **After (v4 compatible)**: Standard require statements
```lua
-- v4 compatible - standard requires
require('common')
local socket = require('socket')
```

## Troubleshooting

### Common Issues and Solutions

1. **"AshitaCore not available" Error**
   - Make sure you're running Ashita v4
   - Check that the addon is loaded after Ashita is fully initialized

2. **"socket library not available" Error**
   - Install luasocket for Ashita v4
   - Make sure the socket library is in the correct Lua path

3. **Chat commands not executing**
   - The addon now tries multiple command execution methods
   - Check the console for specific error messages

4. **Status effects not reading correctly**
   - The addon uses direct StatusServer access for v4
   - Verify entity data is available and properly populated

5. **Event registration failures**
   - The addon includes fallback event registration methods
   - Check console output for specific event registration errors

### Verification Steps

1. **Check Ashita Version**
   - Make sure you're running Ashita v4
   - The addon is now specifically designed for v4 compatibility

2. **Console Output**
   - Look for "[PandaBot] Ashita v4 addon loaded successfully" message
   - Check for any red error messages during load

3. **API Availability**
   - The addon logs which APIs are available: Chat, Memory, Entities, Party
   - All should show "OK" for full functionality

4. **Connection Status**
   - Look for "CONNECTED!" message when the addon connects to the Go server
   - Check for reconnection attempts if connection fails

### Manual Testing

To test if the addon is working correctly:

1. Load the addon: `/addon load pandabot` or `/addon load pandabot_simple`
2. Check console for successful load messages
3. Verify connection to the Go server
4. Test chat monitoring by sending a tell or party message
5. Check that status updates are being sent (every 5 seconds)

## Files Updated

- `cmd/addon/pandabot.lua` - Full-featured version refactored for Ashita v4 compatibility
- `cmd/addon/pandabot_simple.lua` - Working simplified version for Ashita v4
- Removed complex compatibility checks in favor of direct v4 API usage

## Next Steps

If you're still experiencing issues:

1. Check the Ashita console for specific error messages
2. Verify you're running Ashita v4 (the addon is now v4 compatible)
3. Make sure luasocket is properly installed for v4
4. Test with the simple version first (`pandabot_simple.lua`)
5. Check that your Go server is running and accepting connections on port 31337