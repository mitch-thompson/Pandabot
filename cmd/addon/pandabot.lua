addon.name = "Pandabot"
addon.author = "Pandabot"
addon.version = "0.3"
addon.description = "Automated gameplay assistant for Ashita v4"

----------------------------------------------------------------------------------------------------
-- Configuration
----------------------------------------------------------------------------------------------------
local config = {
	host = "127.0.0.1",
	port = 31337,
	status_update_interval = 1000, -- 1 seconds
	connection_timeout = 2, -- 2 seconds
	max_reconnect_attempts = 10,
	base_reconnect_delay = 1000, -- 1 second
	max_message_queue_size = 100,
	command_timeout = 30000, -- 30 seconds
	debug_mode = false
}

-- Ashita v4 requires
require('common')
local socket = require('socket')
require('struct')

-- Simple JSON encoder/decoder for Ashita v4 compatibility
local json = {}
local last_player_pos = { x = 0, y = 0, z = 0 }
local last_position_update = 0
local movement_check_interval = 200  -- Check every ~200ms (adjust if needed)
local last_command_time = 0
local command_buffer_time = 2000 -- 1 second buffer after sending a command
local recv_buffer = ""
local last_ready_sent = 0
local current_action = {
	id = nil,
	command = nil,
	start_time = 0,
	is_casting = false
}

function json.encode(obj)
	local function encode_value(val)
		local val_type = type(val)
		if val_type == "string" then
		-- More comprehensive string escaping for JSON safety
			local escaped = val:gsub('\\', '\\\\'):gsub('"', '\\"'):gsub('\n', '\\n'):gsub('\r', '\\r'):gsub('\t', '\\t')
			-- Escape other control characters that might cause JSON parsing issues
			escaped = escaped:gsub('[\1-\31\127-\255]', function(c)
				return string.format('\\u%04x', string.byte(c))
			end)
			return '"' .. escaped .. '"'
		elseif val_type == "number" then
			return tostring(val)
		elseif val_type == "boolean" then
			return val and "true" or "false"
		elseif val_type == "nil" then
			return "null"
		elseif val_type == "table" then
			local is_array = true
			local max_index = 0
			for k, v in pairs(val) do
				if type(k) ~= "number" or k <= 0 or k ~= math.floor(k) then
					is_array = false
					break
				end
				max_index = math.max(max_index, k)
			end

			if is_array then
				local result = {}
				for i = 1, max_index do
					table.insert(result, encode_value(val[i]))
				end
				return "[" .. table.concat(result, ",") .. "]"
			else
				local result = {}
				for k, v in pairs(val) do
					table.insert(result, encode_value(tostring(k)) .. ":" .. encode_value(v))
				end
				return "{" .. table.concat(result, ",") .. "}"
			end
		else
			error("Cannot encode value of type " .. val_type)
		end
	end

	return encode_value(obj)
end

function json.decode(str)
	local pos = 1

	local function skip_whitespace()
		while pos <= #str and str:sub(pos, pos):match("%s") do
			pos = pos + 1
		end
	end

	local function decode_value()
		skip_whitespace()
		if pos > #str then
			error("Unexpected end of JSON input")
		end

		local char = str:sub(pos, pos)

		if char == '"' then
		-- String
			pos = pos + 1
			local start = pos
			while pos <= #str do
				char = str:sub(pos, pos)
				if char == '"' then
					local result = str:sub(start, pos - 1)
					pos = pos + 1
					return result:gsub('\\"', '"'):gsub('\\\\', '\\'):gsub('\\n', '\n'):gsub('\\r', '\r'):gsub('\\t', '\t')
				elseif char == '\\' then
					pos = pos + 2
				else
					pos = pos + 1
				end
			end
			error("Unterminated string")
		elseif char == '{' then
		-- Object
			pos = pos + 1
			local result = {}
			skip_whitespace()

			if pos <= #str and str:sub(pos, pos) == '}' then
				pos = pos + 1
				return result
			end

			while true do
				local key = decode_value()
				skip_whitespace()
				if pos > #str or str:sub(pos, pos) ~= ':' then
					error("Expected ':' after object key")
				end
				pos = pos + 1
				local value = decode_value()
				result[key] = value

				skip_whitespace()
				if pos > #str then
					error("Unexpected end of JSON input")
				end

				char = str:sub(pos, pos)
				if char == '}' then
					pos = pos + 1
					return result
				elseif char == ',' then
					pos = pos + 1
				else
					error("Expected ',' or '}' in object")
				end
			end
		elseif char == '[' then
		-- Array
			pos = pos + 1
			local result = {}
			skip_whitespace()

			if pos <= #str and str:sub(pos, pos) == ']' then
				pos = pos + 1
				return result
			end

			while true do
				table.insert(result, decode_value())
				skip_whitespace()

				if pos > #str then
					error("Unexpected end of JSON input")
				end

				char = str:sub(pos, pos)
				if char == ']' then
					pos = pos + 1
					return result
				elseif char == ',' then
					pos = pos + 1
				else
					error("Expected ',' or ']' in array")
				end
			end
		elseif char:match("[%d%-]") then
		-- Number
			local start = pos
			if char == '-' then
				pos = pos + 1
			end
			while pos <= #str and str:sub(pos, pos):match("%d") do
				pos = pos + 1
			end
			if pos <= #str and str:sub(pos, pos) == '.' then
				pos = pos + 1
				while pos <= #str and str:sub(pos, pos):match("%d") do
					pos = pos + 1
				end
			end
			return tonumber(str:sub(start, pos - 1))
		elseif str:sub(pos, pos + 3) == "true" then
			pos = pos + 4
			return true
		elseif str:sub(pos, pos + 4) == "false" then
			pos = pos + 5
			return false
		elseif str:sub(pos, pos + 3) == "null" then
			pos = pos + 4
			return nil
		else
			error("Unexpected character: " .. char)
		end
	end

	return decode_value()
end

-- Ashita v4 API references - using proper MemoryManager pattern
local ashita_chat = AshitaCore:GetChatManager()
local memory = AshitaCore:GetMemoryManager()
local party = memory:GetParty()
local bit = require('bit');
local entity = memory:GetEntity()

local sock = nil
local connected = false
local disconnect_at = 0
local reconnect_attempts = 0
local last_reconnect_attempt = 0
local message_queue = {} -- Queue messages during disconnection
local party_buffs = {}

-- Party status tracking (packet-based)
local party_statuses = {}
-- party_statuses[member_index] = {
--     [1] = buffId,
--     [2] = buffId,
--     ...
-- }


-- Color constants for FFXI chat
local COLOR_DEFAULT = string.char(31, 1)
local COLOR_CYAN = string.char(31, 200)
local COLOR_YELLOW = string.char(31, 207)
local COLOR_GREEN = string.char(31, 123)
local COLOR_RED = string.char(31, 255)

----------------------------------------------------------------------------------------------------
-- Helper: Get milliseconds timestamp
----------------------------------------------------------------------------------------------------
local function now_ms()
	return os.clock() * 1000     -- Ashita-safe timing
end

----------------------------------------------------------------------------------------------------
-- Helper: Debug logging
----------------------------------------------------------------------------------------------------
local function debug_log(message)
	if config.debug_mode then
		print(COLOR_CYAN .. '[PandaBot Debug] ' .. COLOR_DEFAULT .. message)
	end
end

----------------------------------------------------------------------------------------------------
-- Helper: Strip FFXI color codes from text for JSON safety
----------------------------------------------------------------------------------------------------
local function strip_color_codes(text)
	if not text then
		return ""
	end

	-- Remove FFXI color control sequences (byte 31 followed by color index)
	-- Also remove other common control characters that cause JSON issues
	local cleaned = text:gsub('\31.', '') -- Remove \31 + any following byte (FFXI color codes)
	cleaned = cleaned:gsub('[\1-\31\127-\255]', '') -- Remove other control characters and high-bit chars

	return cleaned
end

local function send(msg)
	if not sock or not connected then
	-- Queue message for later transmission
		table.insert(message_queue, msg)
		if #message_queue > config.max_message_queue_size then -- Limit queue size
			table.remove(message_queue, 1) -- Remove oldest message
		end
		return false
	end

	local success, data = pcall(json.encode, msg)
	if not success then
		print(COLOR_RED .. '[PandaBot Error] ' .. COLOR_DEFAULT .. 'Failed to encode JSON message')
		return false
	end

	local payload = string.pack(">I4", #data) .. data -- prepend length (4 bytes for JSON)
	local bytes_sent, err = sock:send(payload)

	if not bytes_sent then
		print(COLOR_RED .. '[PandaBot Error] ' .. COLOR_DEFAULT .. 'Failed to send message: ' .. (err or "unknown error"))
		connected = false
		-- Queue the message for retry
		table.insert(message_queue, msg)
		return false
	end

	return true
end

----------------------------------------------------------------------------------------------------
-- Send queued messages after reconnection
----------------------------------------------------------------------------------------------------
local function send_queued_messages()
	while #message_queue > 0 and connected do
		local msg = table.remove(message_queue, 1)
		if not send(msg) then
		-- If send fails, put message back at front of queue
			table.insert(message_queue, 1, msg)
			break
		end
	end
end

----------------------------------------------------------------------------------------------------
-- Attempt to reconnect with exponential backoff
----------------------------------------------------------------------------------------------------
local function attempt_reconnect()
	local current_time = now_ms()

	-- Calculate delay with exponential backoff
	local delay = config.base_reconnect_delay * (2 ^ math.min(reconnect_attempts, 5)) -- Cap at 32 seconds

	if current_time - last_reconnect_attempt < delay then
		return -- Not time to retry yet
	end

	if reconnect_attempts >= config.max_reconnect_attempts then
		print(COLOR_RED .. '[PandaBot Error] ' .. COLOR_DEFAULT .. 'Max reconnection attempts reached. Giving up.')
		return
	end

	last_reconnect_attempt = current_time
	reconnect_attempts = reconnect_attempts + 1

	print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. 'Reconnection attempt ' .. reconnect_attempts .. '/' .. config.max_reconnect_attempts .. '...')

	-- Clean up old socket
	if sock then
		pcall(function() sock:close() end)
		sock = nil
	end

	-- Create new socket and attempt connection
	sock = socket.tcp()
	sock:settimeout(config.connection_timeout) -- Connection timeout

	local success, err = sock:connect(config.host, config.port)
	if success then
		print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. 'Reconnected successfully!')
		connected = true
		reconnect_attempts = 0

		-- Send initial ping
		local ping_msg = {
			type = 1, -- TypePing
			body = nil
		}
		send(ping_msg)

		-- Send initial ready signal
		send_ready_for_action()

		-- Send queued messages
		send_queued_messages()
	else
		print(COLOR_RED .. '[PandaBot Error] ' .. COLOR_DEFAULT .. 'Reconnection failed: ' .. tostring(err))
		connected = false
	end
end
----------------------------------------------------------------------------------------------------
-- Ashita v4 load event
----------------------------------------------------------------------------------------------------
ashita.events.register('load', 'pandabot_load', function()
-- Verify Ashita v4 APIs are available
	if not AshitaCore then
		print('[PandaBot] AshitaCore not available - addon cannot function')
		return
	end

	if not ashita_chat then
		print('[PandaBot] Chat manager not available')
		return
	end

	print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. 'Initializing Ashita v4 addon...')

	-- Log API availability status
	print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. 'API Status:')
	print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. '  Chat: ' .. (ashita_chat and 'OK' or 'FAIL'))
	print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. '  Memory: ' .. (memory and 'OK' or 'FAIL'))
	print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. '  Entity: ' .. (entity and 'OK' or 'FAIL'))
	print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. '  Party: ' .. (party and 'OK' or 'FAIL'))

	-- Register for Ashita v4 events
	if entity and party then
		ashita.events.register('text_in', 'pandabot_chat', handle_chat_message)
		--ashita.events.register('packet_in', 'pandabot_packet', handle_packet_in)
		ashita.events.register('command', 'pandabot_command', handle_command)
		print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. 'Chat monitoring enabled')
		print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. 'Packet monitoring enabled')
		print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. 'Commands enabled: //pandabot status, //pandabot debug')
	else
		print(COLOR_RED .. '[PandaBot Error] ' .. COLOR_DEFAULT .. 'Chat monitoring disabled (missing APIs)')
	end

	-- Initialize connection to Go server
	print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. 'Connecting to ' .. config.host .. ':' .. config.port .. '...')

	local success, err = pcall(function()
		sock = socket.tcp()
		sock:settimeout(config.connection_timeout)  -- Connection timeout

		local connect_success, connect_err = sock:connect(config.host, config.port)
		if not connect_success then
			print(COLOR_RED .. '[PandaBot Error] ' .. COLOR_DEFAULT .. 'Initial connection failed: ' .. tostring(connect_err))
			print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. 'Will attempt automatic reconnection...')
			connected = false
			return
		end

		print(COLOR_GREEN .. '[PandaBot] ' .. COLOR_DEFAULT .. 'CONNECTED!')

		-- Send initial ping using JSON protocol
		local ping_msg = {
			type = 1, -- TypePing
			body = nil
		}
		send(ping_msg)

		-- Send initial ready signal after successful connection
		send_ready_for_action()

		connected = true
		reconnect_attempts = 0 -- Reset reconnect counter on successful connection
	end)

	if not success then
		print(COLOR_RED .. '[PandaBot Error] ' .. COLOR_DEFAULT .. 'Error during initialization: ' .. tostring(err))
		connected = false
	end

	print(COLOR_GREEN .. '[PandaBot] ' .. COLOR_DEFAULT .. 'Ashita v4 addon loaded successfully')
end)

local function hexdump(str, start, len)
	local out = {}
	start = start or 1
	len = len or #str

	for i = start, math.min(start + len - 1, #str) do
		out[#out + 1] = string.format('%02X', str:byte(i))
	end

	return table.concat(out, ' ')
end

ashita.events.register('packet_in', 'pandabot_packet', function(e)
-- Action packet
	if e.id == 0x028 then
		local p = e.data
		-- Offset 5: Category
		-- Category 4: Finish Spell Casting
		-- Category 8: Use Ability
		-- Category 6: Use Weapon Skill
		-- Offset 6: Actor ID (Server ID)
		local actor_id = struct.unpack('I', p, 5 + 1)
		local category = bit.band(bit.rshift(struct.unpack('H', p, 10 + 1), 2), 0x0F)

		local player_id = AshitaCore:GetMemoryManager():GetParty():GetMemberServerId(0)

		if actor_id == player_id then
			debug_log(string.format('Action packet: category=%d', category))
			if category == 4 or category == 8 or category == 6 then
			-- Action completed
				if current_action.id then
					debug_log(string.format('Detected action completion via packet 0x028: %s', current_action.id))
					ashita.tasks.once(0.5, function() send_spell_completion(current_action.id) end)
					current_action.id = nil
					current_action.is_casting = false
				end
			elseif category == 11 then
			-- Action failed / interrupted
				if current_action.id then
					debug_log(string.format('Detected action failure via packet 0x028: %s', current_action.id))
					ashita.tasks.once(0.5, function() send_spell_failure(current_action.id, "Action interrupted or failed") end)
					current_action.id = nil
					current_action.is_casting = false
				end
			end
		end
	end

	if e.id ~= 0x076 then
		return
	end

	local p = e.data
	local plen = #p

	if plen < 16 then
		return
	end

	if config.debug_mode then
		debug_log('Processing 0x076 packet')
	end

	-- Process up to 5 party members (indices 0-4)
	-- Each member occupies 0x30 (48) bytes
	for k = 0, 4 do
	-- Calculate base offset for this member
		local member_base = k * 0x30

		-- Entity index at offset 9 + member_base
		local entity_index_offset = 8 + 1 + member_base

		if entity_index_offset + 1 > plen then
			break
		end

		local entity_index = struct.unpack('H', p, entity_index_offset)

		if entity_index ~= 0 and entity_index ~= nil then
		-- Get the character name using entity index
			local char_name = entity:GetName(entity_index)

			if char_name and char_name ~= "" then
			-- Find which party member index this entity corresponds to
				local party_member_index = nil
				for i = 0, 17 do
					local target_idx = party:GetMemberTargetIndex(i)
					if target_idx == entity_index then
						party_member_index = i
						break
					end
				end

				if party_member_index then
				-- Extract buffs using CurePlease's method
					local buffs = {}
					for i = 1, 32 do
					-- This formula extracts 10-bit buff IDs from the compressed format
					-- Low 8 bits from one location, high 2 bits from bitmask
						local buff_byte_offset = k * 48 + 5 + 16 + i - 1
						local bitmask_offset = k * 48 + 5 + 8 + math.floor((i - 1) / 4)

						if buff_byte_offset < plen and bitmask_offset < plen then
							local low_bits = p:byte(buff_byte_offset)  -- REMOVED + 1
							local bitmask_byte = p:byte(bitmask_offset)  -- REMOVED + 1
							local high_bits = math.floor(bitmask_byte / (4 ^ ((i - 1) % 4))) % 4

							local current_buff = low_bits + (high_bits * 256)

							if current_buff ~= 255 and current_buff ~= 0 then
								table.insert(buffs, current_buff)
							end
						end
					end

					-- Store buffs for this party member
					party_statuses[party_member_index] = buffs

					if config.debug_mode and char_name then
					-- Show raw buff bytes before extraction
						local raw_buff_str = ""
						for i = 1, 32 do
							local buff_byte_offset = k * 48 + 5 + 16 + i - 1
							if buff_byte_offset < plen then
								raw_buff_str = raw_buff_str .. string.format("%02X ", p:byte(buff_byte_offset + 1))
							end
						end
						debug_log(string.format('Member %d raw buff bytes: %s', party_member_index, raw_buff_str))
					end
				end
			end
		end
	end
end)

-- Packet handler for party buffs (0x076)
ashita.events.register('packet_in', 'pandabot_packet_party_buffs', function(e)
	if e.id ~= 0x076 or e.size ~= 0xF4 then return end

	local data = e.data
	for k = 0, 4 do
		local id_pos = k*0x30 + 0x04 + 1
		local id = struct.unpack('I', data, id_pos)
		if id and id ~= 0 then
			party_buffs[id] = {buffs = {}}
			for i = 0, 31 do
				local low = struct.unpack('B', data, k*0x30 + i + 0x14 + 1)
				local mask_byte = struct.unpack('B', data, k*0x30 + i/4 + 0x0C + 1)
				local mask = bit.band(bit.rshift(mask_byte, (i % 4) * 2), 3)
				local buff_id = low + 256 * mask
				if buff_id ~= 255 and buff_id > 0 and buff_id < 1024 then
					table.insert(party_buffs[id].buffs, buff_id)
				end
			end
		end
	end
end)

-- Packet handler for self buffs (0x063 type 9)
ashita.events.register('packet_in', 'pandabot_packet_self_buffs', function(e)
	if e.id ~= 0x063 or e.size < 0x4C then return end

	local data = e.data
	if struct.unpack('B', data, 5) == 9 then  -- Subtype 9: buffs
		local self_id = AshitaCore:GetMemoryManager():GetParty():GetMemberServerId(0)
		if self_id and self_id ~= 0 then
			party_buffs[self_id] = {buffs = {}}
			for i = 0, 31 do
				local pos = 9 + i * 2
				local buff_id = struct.unpack('H', data, pos)
				if buff_id ~= 0xFFFF and buff_id ~= 255 and buff_id > 0 and buff_id < 1024 then
					table.insert(party_buffs[self_id].buffs, buff_id)
				end
			end
		end
	end
end)


----------------------------------------------------------------------------------------------------
-- Get status effects for party member using multiple methods
----------------------------------------------------------------------------------------------------
local function get_party_member_status_effects(member_index)
	local effects = {}

	if party_statuses[member_index] then
		for _, buff in ipairs(party_statuses[member_index]) do
			table.insert(effects, buff)
		end
	end

	return effects
end



----------------------------------------------------------------------------------------------------
-- Ashita v4 command handler
----------------------------------------------------------------------------------------------------
function handle_command(e)
	local args = e.command:lower():split(' ')

	if args[1] ~= '/pandabot' and args[1] ~= '//pandabot' then
		return
	end

	-- Block the command from being sent to the server
	e.blocked = true

	if #args < 2 then
		print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. 'Available commands:')
		print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. '  //pandabot status - Show party status and effects')
		print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. '  //pandabot debug - Toggle debug mode')
		return
	end

	local command = args[2]

	if command == 'status' then
		print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. 'Current party status:')

		for i = 0, 17 do
			local server_id = party:GetMemberServerId(i)
			if server_id > 0 then
				local member_name = party:GetMemberName(i)
				if member_name and member_name ~= "" then
					local hp_percent = party:GetMemberHPPercent(i) or 0
					local mp_percent = party:GetMemberMPPercent(i) or 0
					local status_effects = get_member_status_effects(i)

					local status_str = "None"
					if #status_effects > 0 then
						status_str = table.concat(status_effects, ', ')
					end

					print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT ..
					string.format('  [%d] %s: HP=%d%%, MP=%d%%, Status=[%s]',
						i, member_name, hp_percent, mp_percent, status_str))
				end
			end
		end

	elseif command == 'debug' then
		config.debug_mode = not config.debug_mode
		print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. 'Debug mode: ' .. (config.debug_mode and 'ON' or 'OFF'))

	else
		print(COLOR_RED .. '[PandaBot Error] ' .. COLOR_DEFAULT .. 'Unknown command: ' .. command)
	end
end

----------------------------------------------------------------------------------------------------
-- String split helper function
----------------------------------------------------------------------------------------------------
function string:split(delimiter)
	local result = {}
	local from = 1
	local delim_from, delim_to = string.find(self, delimiter, from)
	while delim_from do
		table.insert(result, string.sub(self, from, delim_from - 1))
		from = delim_to + 1
		delim_from, delim_to = string.find(self, delimiter, from)
	end
	table.insert(result, string.sub(self, from))
	return result
end

----------------------------------------------------------------------------------------------------
-- Ashita v4 chat event handler
----------------------------------------------------------------------------------------------------
function handle_chat_message(e)
	if not connected then
		return
	end

	local base_mode = bit.band(e.mode, 0xFF)  -- Lower 8 bits often hold the core type

	-- Common values: 12 = party, 13/14 = incoming tell (adjust if your testing shows different)
	if base_mode ~= 12 and base_mode ~= 13 and base_mode ~= 14 then
		return  -- Ignore non-party/tell lines
	end

	-- Filter to only capture tell messages (mode 3) and party messages (mode 4)
	local mode = e.mode

	-- Clean the message and sender of FFXI color codes before sending to server
	local clean_message = strip_color_codes(e.message)
	local clean_sender = strip_color_codes(e.sender or "Unknown")

	-- Capture only tell and party messages with cleaned text
	local chat_msg = {
		type = 20, -- TypeChatLine
		body = {
			mode = base_mode,
			sender = clean_sender,
			message = clean_message,
			timestamp = os.time()
		}
	}
	send(chat_msg)

	-- Debug: Print what we're capturing (use cleaned versions)
	debug_log('Chat captured - Mode: ' .. mode .. ', Sender: ' .. clean_sender .. ', Message: ' .. clean_message)
end

----------------------------------------------------------------------------------------------------
-- Ashita v4 periodic status update (called every frame)
----------------------------------------------------------------------------------------------------
local last_status_update = 0

ashita.events.register('d3d_present', 'pandabot_present', function()
-- Handle reconnection if disconnected
	if not connected then
		attempt_reconnect()
		return
	end

	-- Send periodic status updates (only if we have the required APIs)
	if entity and party then
		local current_time = now_ms()
		if current_time - last_status_update >= config.status_update_interval then
			send_status_update()
			last_status_update = current_time
		end
	end

	-- Send ready if idle and cooldown passed
	if connected and not current_action.is_casting and (now_ms() - last_ready_sent > 500) then
		send_ready_for_action()
	end

	-- Check for incoming messages from server
	receive_messages()

	-- Process command queue
	--process_command_queue()

	-- Process pending spells for completion tracking
	process_pending_spells()
end)

----------------------------------------------------------------------------------------------------
-- Job ID to name mapping (FFXI job IDs)
----------------------------------------------------------------------------------------------------
local job_names = {
	[0] = "NONE",
	[1] = "WAR", [2] = "MNK", [3] = "WHM", [4] = "BLM", [5] = "RDM", [6] = "THF",
	[7] = "PLD", [8] = "DRK", [9] = "BST", [10] = "BRD", [11] = "RNG", [12] = "SAM",
	[13] = "NIN", [14] = "DRG", [15] = "SMN", [16] = "BLU", [17] = "COR", [18] = "PUP",
	[19] = "DNC", [20] = "SCH", [21] = "GEO", [22] = "RUN"
}

function get_job_name(job_id)
	return job_names[job_id] or "UNK"
end

local function get_member_status_effects(member_index)
	if party_statuses[member_index] and #party_statuses[member_index] > 0 then
		return party_statuses[member_index]
	end

	local effects = {}
	local party_mgr = AshitaCore:GetMemoryManager():GetParty()
	if not party_mgr then
		return effects
	end

	local icons_lo = party_mgr:GetStatusIcons(member_index)
	local icons_hi = party_mgr:GetStatusIconsBitMask(member_index)

	for j = 0, 31 do
		local high_bits = 0
		if j < 16 then
			high_bits = bit.lshift(bit.band(bit.rshift(icons_hi, 2 * j), 3), 8)
		else
			local buffer = math.floor(icons_hi / 0xFFFFFFFF)
			high_bits = bit.lshift(bit.band(bit.rshift(buffer, 2 * (j - 16)), 3), 8)
		end

		local buff_id = icons_lo[j + 1] + high_bits
		if buff_id ~= 0 and buff_id ~= 255 then
			table.insert(effects, buff_id)
		end
	end

	return effects
end

----------------------------------------------------------------------------------------------------
-- Send status update using Ashita v4 APIs
----------------------------------------------------------------------------------------------------
-- Updated send_status_update function
function send_status_update()
	local party_mgr = AshitaCore:GetMemoryManager():GetParty()
	local body = {
		Timestamp = os.time(),
		PlayerName = party_mgr:GetMemberName(0),
		Zone = party_mgr:GetMemberZone(0),
		Members = {}
	}

	for i = 0, 17 do
		local name = party_mgr:GetMemberName(i)
		if name ~= nil and name ~= "" then  -- Loosened check for inclusion
			local server_id = party_mgr:GetMemberServerId(i)
			local buffs_list = {}
			if party_buffs[server_id] and party_buffs[server_id].buffs then
				buffs_list = party_buffs[server_id].buffs
				table.sort(buffs_list)
			end

			local member = {
				Name = name,
				HPPercent = party_mgr:GetMemberHPPercent(i),
				MPPercent = party_mgr:GetMemberMPPercent(i),
				HPActual = party_mgr:GetMemberHP(i),
				MPActual = party_mgr:GetMemberMP(i),
				Job = party_mgr:GetMemberMainJob(i),
				Zone = party_mgr:GetMemberZone(i),
				StatusEffects = buffs_list  -- Sorted array of buff IDs
			}
			table.insert(body.Members, member)
		end
	end

	local msg = {
		Type = 21,
		Body = body
	}

	-- Send JSON with length prefix
	local json_lib = require('json')
	local json_str = json_lib.encode(msg)
	local length_packed = struct.pack('>I', #json_str)
	sock:send(length_packed .. json_str)
end



----------------------------------------------------------------------------------------------------
-- Receive messages from Go server
----------------------------------------------------------------------------------------------------
function receive_messages()
	if not sock then
		return
	end

	local success, err = pcall(function()
	-- Set socket to non-blocking to avoid freezing the game
		sock:settimeout(0)

		-- Try to read length prefix (4 bytes)
		local length_data, err = sock:receive(4)
		if not length_data then
			if err == "closed" then
				print(COLOR_RED .. '[PandaBot Error] ' .. COLOR_DEFAULT .. 'Connection closed by server')
				connected = false
			end
			return -- No data available or connection closed
		end

		-- Unpack message length
		local length = string.unpack(">I4", length_data)
		if length > 1024*1024 then -- 1MB limit
			print(COLOR_RED .. '[PandaBot Error] ' .. COLOR_DEFAULT .. 'Message too large: ' .. length .. ' bytes')
			return
		end

		-- Read message data
		local message_data, err = sock:receive(length)
		if not message_data then
			if err == "closed" then
				print(COLOR_RED .. '[PandaBot Error] ' .. COLOR_DEFAULT .. 'Connection closed while reading message data')
				connected = false
			else
				print(COLOR_RED .. '[PandaBot Error] ' .. COLOR_DEFAULT .. 'Failed to receive message data: ' .. tostring(err))
			end
			return
		end

		-- Parse JSON message
		local success, message = pcall(json.decode, message_data)
		if not success then
			print(COLOR_RED .. '[PandaBot Error] ' .. COLOR_DEFAULT .. 'Failed to parse JSON message: ' .. tostring(message))
			return
		end

		-- Handle the message
		handle_server_message(message)
	end)

	if not success then
		print(COLOR_RED .. '[PandaBot Error] ' .. COLOR_DEFAULT .. 'Error in receive_messages: ' .. tostring(err))
		connected = false
	end
end

----------------------------------------------------------------------------------------------------
-- Handle messages from Go server
----------------------------------------------------------------------------------------------------
function handle_server_message(message)
	if not message or not message.type then
		return
	end

	local msg_type = message.type

	if msg_type == 2 then -- TypePong
		print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. 'Received pong from server')
	elseif msg_type == 10 then -- TypeExecuteCommand
		handle_execute_command(message.body)
	elseif msg_type == 40 then -- TypeReadyToCast (Deprecated)
	-- Server no longer polls, but we'll respond just in case of version mismatch
		handle_ready_to_cast_check(message.body)
	else
		print(COLOR_RED .. '[PandaBot Error] ' .. COLOR_DEFAULT .. 'Unknown message type: ' .. msg_type)
	end
end

----------------------------------------------------------------------------------------------------
-- Handle execute command from server
----------------------------------------------------------------------------------------------------
function handle_execute_command(command_data)
	if not command_data or not command_data.command then
		print(COLOR_RED .. '[PandaBot Error] ' .. COLOR_DEFAULT .. 'Invalid command data received')
		return
	end

	local priority = command_data.priority or 5 -- Default priority
	print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. 'Executing command (priority ' .. priority .. '): ' .. command_data.command)

	-- Execute the command immediately
	local success = execute_command(command_data.command, command_data.id)

	-- Send error report if command failed
	if not success and command_data.id then
		send_spell_failure(command_data.id, "Command execution failed")
	end
end

----------------------------------------------------------------------------------------------------
-- Handle ready to cast check from server
----------------------------------------------------------------------------------------------------
function handle_ready_to_cast_check(check_data)
	if not check_data or not check_data.command_id then
		return
	end

	local command_id = check_data.command_id
	local is_ready = true
	local reason = ""

	-- Update player position periodically (in d3d_present or here is fine, but here avoids extra globals)
	local current_time = now_ms()
	if current_time - last_position_update >= movement_check_interval then
		local entity_mgr = AshitaCore:GetMemoryManager():GetEntity()
		local player_index = party:GetMemberTargetIndex(0)
		if player_index and player_index > 0 then
			local current_x = entity_mgr:GetLocalPositionX(player_index) or 0
			local current_y = entity_mgr:GetLocalPositionY(player_index) or 0
			local current_z = entity_mgr:GetLocalPositionZ(player_index) or 0

			-- Small threshold for floating-point jitter
			if math.abs(current_x - last_player_pos.x) > 0.01 or
			math.abs(current_y - last_player_pos.y) > 0.01 or
			math.abs(current_z - last_player_pos.z) > 0.01 then
				last_player_pos = { x = current_x, y = current_y, z = current_z }
			end

			last_position_update = current_time
		end
	end

	-- Check if player is casting (via status effect buff - reliable)
	local is_casting = false
	local player_buffs = AshitaCore:GetMemoryManager():GetPlayer():GetBuffs()
	if player_buffs then
		for _, buff_id in ipairs(player_buffs) do
			if buff_id == 2 or buff_id == 173 then  -- 2 = spell casting, 173 = ability/WS
				is_casting = true
				break
			end
		end
	end

	if current_action.is_casting then
	-- Double check with memory if we think we are casting but memory says no
		local player_buffs = AshitaCore:GetMemoryManager():GetPlayer():GetBuffs()
		local memory_says_busy = false
		if player_buffs then
			for _, buff_id in ipairs(player_buffs) do
				if buff_id == 2 or buff_id == 173 then
					memory_says_busy = true
					break
				end
			end
		end

		-- If 10 seconds passed, assume something went wrong and we are not casting anymore
		if current_time - current_action.start_time > 10000 then
			debug_log('Force clearing current_action after 10s timeout')
			current_action.is_casting = false
			current_action.id = nil
		elseif memory_says_busy then
			is_casting = true
		else
		-- Memory says we are not busy, but we thought we were.
		-- Give it 2 seconds to definitely register the start of casting
			if current_time - current_action.start_time > 2000 then
				debug_log('Memory says not casting, clearing current_action after 2s grace period')
				current_action.is_casting = false
				current_action.id = nil
			else
				is_casting = true
			end
		end
	end

	-- Detect moving by position change
	local is_moving = false
	local entity_mgr = AshitaCore:GetMemoryManager():GetEntity()
	local player_index = party:GetMemberTargetIndex(0)
	if player_index and player_index > 0 then
		local current_x = entity_mgr:GetLocalPositionX(player_index) or 0
		local current_y = entity_mgr:GetLocalPositionY(player_index) or 0
		local current_z = entity_mgr:GetLocalPositionZ(player_index) or 0

		if math.abs(current_x - last_player_pos.x) > 0.05 or
		math.abs(current_y - last_player_pos.y) > 0.05 or
		math.abs(current_z - last_player_pos.z) > 0.05 then
			is_moving = true
		end
	end

	if is_casting then
		is_ready = false
		reason = "casting"
	elseif is_moving then
		is_ready = false
		reason = "moving"
	elseif current_time - last_command_time < command_buffer_time then
		is_ready = false
		reason = "busy (recently sent command)"
	end

	local response = {
		type = 41, -- TypeReadyResponse
		body = {
			command_id = command_id,
			is_ready = is_ready,
			reason = reason
		}
	}
	send(response)
end

----------------------------------------------------------------------------------------------------
-- Execute command using Ashita v4 command injection
----------------------------------------------------------------------------------------------------
function execute_command(command, command_id)
	if not command or command == "" then
		return false
	end

	last_command_time = now_ms()

	-- Track current action
	current_action.id = command_id
	current_action.command = command
	current_action.start_time = last_command_time
	current_action.is_casting = true

	local success, err = pcall(function()
	-- Use Ashita v4 command system
		ashita_chat:QueueCommand(1, command)

		-- Send spell completion notification after a short delay
		-- In a real implementation, this would be triggered by actual spell completion events
		-- For now, we'll simulate completion after command execution
		if command_id then
		-- Schedule completion notification
			schedule_spell_completion(command_id, command)
		end
	end)

	if not success then
		print(COLOR_RED .. '[PandaBot Error] ' .. COLOR_DEFAULT .. 'Failed to execute command: ' .. tostring(err))

		-- Send failure notification
		if command_id then
			send_spell_failure(command_id, tostring(err))
		end

		return false
	end

	return true
end

----------------------------------------------------------------------------------------------------
-- Schedule spell completion notification
----------------------------------------------------------------------------------------------------
local pending_spells = {}

function schedule_spell_completion(command_id, command)
-- Store the spell for completion tracking
	pending_spells[command_id] = {
		command = command,
		start_time = now_ms(),
		timeout = 10000 -- 10 second timeout for spell completion
	}

	debug_log('Scheduled completion tracking for command: ' .. command_id .. ' (' .. command .. ')')
end

function send_ready_for_action()
	if not connected then return end
	local msg = {
		type = 42,
		body = {
			last_command_id = current_action.last_id or "",
			last_status = current_action.last_status or "idle",
			timestamp = now_ms()
		}
	}
	send(msg)
	last_ready_sent = now_ms()
	debug_log("Sent ready for action")
end

----------------------------------------------------------------------------------------------------
-- Send spell completion notification
----------------------------------------------------------------------------------------------------
function send_spell_completion(command_id)
	if not connected then return end
	local msg = { type = 31, body = { command_id = command_id, timestamp = os.time() } }
	send(msg)
	--debug_log('Sent completion: ' .. command_id)
	current_action.is_casting = false
	current_action.id = nil
	send_ready_for_action()
end

----------------------------------------------------------------------------------------------------
-- Send spell failure notification
----------------------------------------------------------------------------------------------------
function send_spell_failure(command_id, error_msg)
	if not connected then return end
	local msg = { type = 32, body = { command_id = command_id, error = error_msg, timestamp = os.time() } }
	send(msg)
	debug_log('Sent failure: ' .. command_id)
	current_action.is_casting = false
	current_action.id = nil
	send_ready_for_action()
end

----------------------------------------------------------------------------------------------------
-- Process pending spells and check for completion/timeout
----------------------------------------------------------------------------------------------------
function process_pending_spells()
	local current_time = now_ms()

	for command_id, spell_data in pairs(pending_spells) do
		local elapsed = current_time - spell_data.start_time

		-- Use packet-based completion primarily, but keep a timeout fallback
		if elapsed >= spell_data.timeout then
		-- Timeout - send failure
			send_spell_failure(command_id, "Spell casting timed out")
			pending_spells[command_id] = nil
			if current_action.id == command_id then
				current_action.id = nil
				current_action.is_casting = false
			end
		end
	end
end

local function process_network()
	if not connected or not sock then return end

	sock:settimeout(0)
	local data, err, partial = sock:receive(4096)
	if data then
		recv_buffer = recv_buffer .. data
	elseif partial and #partial > 0 then
		recv_buffer = recv_buffer .. partial
	elseif err and err ~= "timeout" then
		print('[PandaBot] Connection lost: ' .. err)
		connected = false
		sock:close()
		sock = nil
		return
	end

	-- Process complete lines
	local nl = recv_buffer:find("\n")
	while nl do
		local line = recv_buffer:sub(1, nl-1)
		recv_buffer = recv_buffer:sub(nl+1)

		local json_part = line:match("|(.+)") or line
		local ok, msg = pcall(json.decode, json_part)
		if ok and msg and msg.type then
			if msg.type == 10 then  -- ExecuteCommand
				local cmd = msg.body.command
				local id = msg.body.id or ""
				execute_command(cmd, id)
			end
		-- Add other handlers here (e.g., status request, etc.)
		end

		nl = recv_buffer:find("\n")
	end
end

----------------------------------------------------------------------------------------------------
-- Ashita v4 unload handler
----------------------------------------------------------------------------------------------------
ashita.events.register('unload', 'pandabot_unload', function()
	print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. 'Unloading addon...')

	if sock then
		pcall(function() sock:close() end)
		sock = nil
	end

	connected = false

	-- Unregister events
	ashita.events.unregister('text_in', 'pandabot_chat')
	ashita.events.unregister('packet_in', 'pandabot_packet')
	ashita.events.unregister('command', 'pandabot_command')
	ashita.events.unregister('d3d_present', 'pandabot_present')

	print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. 'Addon unloaded')
end)