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
	status_update_interval = 5000, -- 5 seconds
	connection_timeout = 2, -- 2 seconds
	max_reconnect_attempts = 10,
	base_reconnect_delay = 1000, -- 1 second
	max_message_queue_size = 100,
	max_command_queue_size = 50,
	command_timeout = 30000, -- 30 seconds
	debug_mode = false
}

-- Ashita v4 requires
require('common')
local socket = require('socket')
require('struct')

-- Simple JSON encoder/decoder for Ashita v4 compatibility
local json = {}

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
local entity = memory:GetEntity()

local sock = nil
local connected = false
local disconnect_at = 0
local reconnect_attempts = 0
local last_reconnect_attempt = 0
local message_queue = {} -- Queue messages during disconnection

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
	-- Only log interesting packets
	if e.id == 13 or e.id == 14 or e.id == 118 then
		print(string.format(
			'[PandaBot][DEBUG] packet id=%d (0x%02X) len=%d',
			e.id, e.id, #e.data
		))
	end
end)

--ashita.events.register('packet_in', 'pandabot_packet', function(e)
--	if e.id ~= 0x076 then
--		return
--	end
--
--	if not config.debug_mode then
--		return
--	end
--
--	local p = e.data
--	local plen = #p
--
--	print('[PandaBot][DEBUG] ================================')
--	print(string.format(
--		'[PandaBot][DEBUG] Packet 0x076 received (len=%d)',
--		plen
--	))
--
--	-- Dump first 64 bytes
--	print('[PandaBot][DEBUG] Raw bytes (0x01–0x40):')
--	print(hexdump(p, 1, 64))
--
--	-- Attempt to read party count
--	local party_count = struct.unpack('B', p, 0x05)
--	print(string.format(
--		'[PandaBot][DEBUG] Parsed party_count @0x05 = %d',
--		party_count
--	))
--
--	local offset = 0x09
--
--	for member_index = 0, math.min(party_count - 1, 5) do
--		print(string.format(
--			'[PandaBot][DEBUG] ---- Party member %d ----',
--			member_index
--		))
--
--		local buffs = {}
--
--		for i = 1, 32 do
--			if offset + 1 > plen then
--				print('[PandaBot][DEBUG] Offset exceeded packet length!')
--				break
--			end
--
--			local buff = struct.unpack('H', p, offset)
--			offset = offset + 2
--
--			if buff ~= 0 then
--				table.insert(buffs, buff)
--			end
--		end
--
--		print(string.format(
--			'[PandaBot][DEBUG] Buffs: %s',
--			(#buffs > 0) and table.concat(buffs, ', ') or 'NONE'
--		))
--	end
--
--	print('[PandaBot][DEBUG] ================================')
--end)





----------------------------------------------------------------------------------------------------
-- Ashita v4 packet handler for status effects (0x076 packets)
----------------------------------------------------------------------------------------------------
--function handle_packet_in(e)
	-- For now, we'll use a simpler approach and rely on target-based status checking
	-- The 0x076 packet parsing is complex and varies between different game versions
	-- We'll implement a more reliable method using entity targeting
--end


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
					local status_effects = get_party_member_status_effects(i)
					
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
	
	-- For party chat, sender might be embedded in the message (format: "PlayerName: message")
	if base_mode == 12 and clean_sender == "Unknown" and clean_message then
		local sender_from_msg, actual_message = clean_message:match("^([^:]+):%s*(.*)$")
		if sender_from_msg and actual_message then
			clean_sender = sender_from_msg:gsub("^%s*(.-)%s*$", "%1") -- Trim whitespace
			clean_message = actual_message
			debug_log('Extracted sender from party message: ' .. clean_sender)
		end
	end
	
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

	-- Check for incoming messages from server
	receive_messages()

	-- Process command queue
	process_command_queue()
	
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

----------------------------------------------------------------------------------------------------
-- Send status update using Ashita v4 APIs
----------------------------------------------------------------------------------------------------
function send_status_update()
	if not entity or not party then
		return -- APIs not initialized yet
	end

	local success, err = pcall(function()
		local party_members = {}

		-- Get party member information using proper Ashita v4 MemoryManager pattern
		for i = 0, 17 do -- Check all possible party slots (0-17)
			-- Check if member exists and is in same zone (v4 pattern)
			local server_id = party:GetMemberServerId(i)
			if server_id > 0 then
				-- Get member name
				local member_name = party:GetMemberName(i)
				if member_name and member_name ~= "" then
					-- Get HP/MP using proper v4 methods (both actual and percentage)
					local hp_percent = party:GetMemberHPPercent(i)
					local mp_percent = party:GetMemberMPPercent(i)
					local hp_actual = party:GetMemberHP(i)     -- Current HP
					local mp_actual = party:GetMemberMP(i)     -- Current MP
					
					local hp_max = 0
					local mp_max = 0
					
					if hp_percent > 0 then
						hp_max = math.floor(hp_actual * 100 / hp_percent + 0.5)
					end
					if mp_percent > 0 then
						mp_max = math.floor(mp_actual * 100 / mp_percent + 0.5)
					end


					-- Get status effects using improved multi-method approach
					local status_effects = get_party_member_status_effects(i)
					
					-- Debug logging for status effects
					if #status_effects > 0 then
						debug_log('Got status effects for ' .. member_name .. ': ' .. table.concat(status_effects, ', '))
					else
						debug_log('No status effects found for ' .. member_name)
					end

					-- Calculate distance if needed
					local distance = 0
					local target_index = party:GetMemberTargetIndex(i)
					if target_index and target_index > 0 then
						distance = entity:GetDistance(target_index) or 0
					end

					-- Get job information
					local main_job = party:GetMemberMainJob(i) or 0
					local zone = party:GetMemberZone(i) or 0

					table.insert(party_members, {
						name = member_name,
						hp_percent = hp_percent or 0,
						mp_percent = mp_percent or 0,
						hp_actual = hp_actual or 0,
						hp_max = hp_max,
						mp_actual = mp_actual or 0,
						mp_max = mp_max,
						status_effects = status_effects,
						job = main_job,
						distance = distance,
						zone = zone,
						last_update = os.time()
					})
				end
			end
		end

		-- Get player info using v4 methods (player is always index 0)
		local player_hp_percent = party:GetMemberHPPercent(0) or 0
		local player_mp_percent = party:GetMemberMPPercent(0) or 0
		local player_zone = party:GetMemberZone(0) or 0
		
		-- Get actual current HP/MP values from Ashita v4 MemoryManager
		-- Use the player manager to get current values, not percentages
		local party = AshitaCore:GetMemoryManager():GetParty()

		local player_hp_actual = party:GetMemberHP(0)     -- Current HP (actual)
		local player_mp_actual = party:GetMemberMP(0)     -- Current MP (actual)
		
		-- Get job levels
		local job_levels = {}
		local main_job_id = party:GetMemberMainJob(0) or 0
		local sub_job_id = party:GetMemberSubJob(0) or 0
		local main_job_level = party:GetMemberMainJobLevel(0) or 1
		local sub_job_level = party:GetMemberSubJobLevel(0) or 1
		
		-- Convert job IDs to names and add to job_levels map
		local main_job_name = get_job_name(main_job_id)
		local sub_job_name = get_job_name(sub_job_id)
		
		if main_job_name then
			job_levels[main_job_name] = main_job_level
		end
		if sub_job_name and sub_job_level > 0 then
			job_levels[sub_job_name] = sub_job_level
		end

		local status_msg = {
			type = 21, -- TypeStatusUpdate
			body = {
				timestamp = os.time(),
				party_members = party_members,
				player_mp = player_mp_actual,  -- Send actual MP, not percentage
				player_hp = player_hp_actual,  -- Send actual HP, not percentage
				zone = player_zone,
				job_levels = job_levels  -- Add job levels
			}
		}

		send(status_msg)
	end)

	if not success then
		print(COLOR_RED .. '[PandaBot Error] ' .. COLOR_DEFAULT .. 'Error in status update: ' .. tostring(err))
	end
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
	else
		print(COLOR_RED .. '[PandaBot Error] ' .. COLOR_DEFAULT .. 'Unknown message type: ' .. msg_type)
	end
end

-- Command queue for priority-based execution
local command_queue = {}

----------------------------------------------------------------------------------------------------
-- Handle execute command from server
----------------------------------------------------------------------------------------------------
function handle_execute_command(command_data)
	if not command_data or not command_data.command then
		print(COLOR_RED .. '[PandaBot Error] ' .. COLOR_DEFAULT .. 'Invalid command data received')
		return
	end

	-- Add command to priority queue
	local priority = command_data.priority or 5 -- Default priority
	local command_entry = {
		command = command_data.command,
		target = command_data.target,
		priority = priority,
		id = command_data.id,
		timestamp = os.time(),
		timeout = command_data.timeout or config.command_timeout -- Default timeout from config
	}

	table.insert(command_queue, command_entry)

	-- Limit command queue size
	if #command_queue > config.max_command_queue_size then
		table.remove(command_queue, #command_queue) -- Remove lowest priority command
		debug_log('Command queue full, removed oldest command')
	end

	-- Sort queue by priority (higher priority first)
	table.sort(command_queue, function(a, b)
		return a.priority > b.priority
	end)

	print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. 'Queued command (priority ' .. priority .. '): ' .. command_data.command)
	debug_log('Command queue size: ' .. #command_queue)
end

----------------------------------------------------------------------------------------------------
-- Process command queue (called from main loop)
----------------------------------------------------------------------------------------------------
function process_command_queue()
	if #command_queue == 0 then
		return
	end

	-- Get highest priority command
	local command_entry = table.remove(command_queue, 1)

	-- Check if command has timed out
	local current_time = os.time() * 1000 -- Convert to milliseconds
	if current_time - (command_entry.timestamp * 1000) > command_entry.timeout then
		print(COLOR_RED .. '[PandaBot Error] ' .. COLOR_DEFAULT .. 'Command timed out: ' .. command_entry.command)

		if command_entry.id then
			send_spell_failure(command_entry.id, "Command timed out")
		end
		return
	end

	print(COLOR_CYAN .. '[PandaBot] ' .. COLOR_DEFAULT .. 'Executing: ' .. command_entry.command)

	-- Execute the command with ID for completion tracking
	local success = execute_command(command_entry.command, command_entry.id)

	-- Send error report if command failed
	if not success and command_entry.id then
		send_spell_failure(command_entry.id, "Command execution failed")
	end
end

----------------------------------------------------------------------------------------------------
-- Execute command using Ashita v4 command injection
----------------------------------------------------------------------------------------------------
function execute_command(command, command_id)
	if not command or command == "" then
		return false
	end

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

----------------------------------------------------------------------------------------------------
-- Send spell completion notification
----------------------------------------------------------------------------------------------------
function send_spell_completion(command_id)
	if not connected then
		return
	end
	
	local completion_msg = {
		type = 31, -- TypeSpellComplete
		body = {
			command_id = command_id,
			timestamp = os.time()
		}
	}
	
	send(completion_msg)
	debug_log('Sent spell completion for command: ' .. command_id)
end

----------------------------------------------------------------------------------------------------
-- Send spell failure notification
----------------------------------------------------------------------------------------------------
function send_spell_failure(command_id, error_message)
	if not connected then
		return
	end
	
	local failure_msg = {
		type = 32, -- TypeSpellFailed
		body = {
			command_id = command_id,
			error = error_message,
			timestamp = os.time()
		}
	}
	
	send(failure_msg)
	debug_log('Sent spell failure for command: ' .. command_id .. ' (error: ' .. error_message .. ')')
end

----------------------------------------------------------------------------------------------------
-- Process pending spells and check for completion/timeout
----------------------------------------------------------------------------------------------------
function process_pending_spells()
	local current_time = now_ms()
	
	for command_id, spell_data in pairs(pending_spells) do
		local elapsed = current_time - spell_data.start_time
		
		-- For now, simulate spell completion after 3 seconds
		-- In a real implementation, this would be based on actual game events
		if elapsed >= 3000 then -- 3 seconds
			send_spell_completion(command_id)
			pending_spells[command_id] = nil
		elseif elapsed >= spell_data.timeout then
			-- Timeout - send failure
			send_spell_failure(command_id, "Spell casting timed out")
			pending_spells[command_id] = nil
		end
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