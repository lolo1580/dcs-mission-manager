--[[
  DCS Manager — Hooks/dcsmanager.lua
  ------------------------------------------------------------------
  Sends to the backend, over TCP/JSON (one JSON line per message):
    - game events (kill, crash, eject, takeoff, landing...);
    - the list of connected players with their statistics;
    - the chat;
    - the mission start and end.

  Place this in: Saved Games\DCS\Scripts\Hooks\dcsmanager.lua

  ⚠️  DCS loads ALL .lua files from Hooks/ and sorts them by name.
      This file is additive: it does not overwrite any other hook.

  Prerequisite: Config/dcsmanager.cfg present in Saved Games\DCS\Config\

  Options (dcsmanager.cfg):
    dcsmanager_host            backend address (default 127.0.0.1)
    dcsmanager_tcp_port        backend TCP port (default 7779)
    dcsmanager_enabled         enable/disable (default true)
    dcsmanager_players_interval players send interval in seconds (default 5.0)
]]

-- >>> DCSMANAGER-BEGIN (managed block — do not edit by hand) >>>
do
  local function say(msg)
    if net and net.log then net.log("DCSMANAGER: " .. tostring(msg)) end
  end
  ---------------------------------------------------------------------------
  -- Configuration
  ---------------------------------------------------------------------------
  local host, tcpPort = "127.0.0.1", 7779
  local enabled = true
  local playersInterval = 5.0

  if lfs and lfs.writedir then
    local cfgPath = lfs.writedir() .. "Config/dcsmanager.cfg"
    local ok, chunk = pcall(loadfile, cfgPath)
    if ok and chunk then
      local env = {}
      setmetatable(env, { __index = _G })
      setfenv(chunk, env)
      pcall(chunk)
      host = env.dcsmanager_host or host
      tcpPort = env.dcsmanager_tcp_port or tcpPort
      playersInterval = env.dcsmanager_players_interval or playersInterval
      if env.dcsmanager_enabled ~= nil then enabled = env.dcsmanager_enabled end
    end
  end

  if not enabled then
    say("hooks disabled by configuration")
    return
  end

  local socket_ok, socket = pcall(require, "socket")
  if not socket_ok then
    say("LuaSocket not found, hooks disabled")
    return
  end

  -- Base64 lives in LuaSocket's "mime" module (mime.b64), not in "socket".
  -- Requiring it separately is what makes debrief transfer work at all; the
  -- previous attempt to call socket.base64 always failed and the debrief was
  -- silently never sent.
  local mime_ok, mime = pcall(require, "mime")

  ---------------------------------------------------------------------------
  -- Persistent TCP connection (automatic reconnection)
  ---------------------------------------------------------------------------
  local conn

  local function connect()
    local c, err = socket.tcp()
    if not c then return false end
    c:settimeout(0.5)               -- never block the simulator
    local okConn, cerr = c:connect(host, tcpPort)
    if not okConn and cerr ~= "already connected" then
      c:close()
      return false
    end
    c:setoption("tcp-nodelay", true)
    conn = c
    say("connected to backend " .. host .. ":" .. tostring(tcpPort))
    return true
  end

  local function jsonEscape(s)
    return tostring(s):gsub('[%z\1-\31\\"]', function(c)
      if c == '"' then return '\\"' end
      if c == '\\' then return '\\\\' end
      if c == '\n' then return '\\n' end
      if c == '\r' then return '\\r' end
      if c == '\t' then return '\\t' end
      return string.format('\\u%04x', string.byte(c))
    end)
  end

  -- Sends every byte of data, never just the first part.
  --
  -- LuaSocket's send() may write only a fraction of the buffer and still return
  -- success. The previous code checked only "not ok", so a partial write looked
  -- like a success and the rest of the line was dropped: the backend never saw
  -- the closing newline and discarded the message as incomplete. A 25 KB debrief
  -- line is exactly the kind of message that gets cut this way.
  local function sendAll(data)
    local n = #data
    local i = 1
    while i <= n do
      local last, err, partial = conn:send(data, i)
      if last and last >= i then
        i = last + 1
      elseif type(partial) == "number" and partial >= i then
        i = partial + 1
      else
        return false, err or "send failed"
      end
    end
    return true
  end

  local sendFailures = 0
  local function sendLine(line)
    if not conn and not connect() then
      return false
    end
    local ok, err = sendAll(line .. "\n")
    if not ok then
      -- Report it: silence is what hid the loss of the debrief and of the
      -- mission-end message.
      if sendFailures < 3 then
        say("TCP send failed: " .. tostring(err))
      end
      sendFailures = sendFailures + 1
      pcall(function() conn:close() end)
      conn = nil
      return false
    end
    return true
  end

  -- Serializes a Lua value to JSON (simple types, lists, tables).
  local function toJson(v)
    local t = type(v)
    if t == "nil" then return "null" end
    if t == "number" then
      if v ~= v or v == math.huge or v == -math.huge then return "null" end
      return string.format("%.6g", v)
    end
    if t == "boolean" then return v and "true" or "false" end
    if t == "string" then return '"' .. jsonEscape(v) .. '"' end
    if t == "table" then
      -- An empty table is ambiguous in Lua. Every table this function emits
      -- when empty is a list (the player roster, an event's arguments), and the
      -- backend decodes those as JSON arrays: emitting {} made it reject the
      -- whole line, losing the player roster whenever no player was connected.
      if next(v) == nil then return "[]" end
      -- List vs object detection.
      local isArray = #v > 0
      local parts = {}
      if isArray then
        for _, item in ipairs(v) do
          parts[#parts + 1] = toJson(item)
        end
        return "[" .. table.concat(parts, ",") .. "]"
      end
      for k, item in pairs(v) do
        parts[#parts + 1] = '"' .. jsonEscape(tostring(k)) .. '":' .. toJson(item)
      end
      return "{" .. table.concat(parts, ",") .. "}"
    end
    return '"' .. jsonEscape(tostring(v)) .. '"'
  end

  -- Hooks run in the GUI Lua state, where the export API lives in the Export.
  -- namespace and NOT as a global: `LoGetModelTime` is nil here, so the old
  -- `(LoGetModelTime and LoGetModelTime()) or 0` was always 0 and every periodic
  -- timer (players, slots, commands) was stuck. Resolve it through Export.
  local function modelTime()
    local fn = (Export and Export.LoGetModelTime) or rawget(_G, "LoGetModelTime")
    if not fn then return 0 end
    local ok, v = pcall(fn)
    if ok and type(v) == "number" then return v end
    return 0
  end

  -- The running theatre, so a mission is recorded on the map it is actually
  -- flown on instead of defaulting to Caucasus. Sim.getCurrentMission returns
  -- the loaded mission table; the id sits in `.theatre` (or `.mission.theatre`).
  local function missionTheatre()
    if not (Sim and Sim.getCurrentMission) then return "" end
    local ok, m = pcall(Sim.getCurrentMission)
    if not ok or type(m) ~= "table" then return "" end
    local t = m.theatre
    if type(t) ~= "string" and type(m.mission) == "table" then
      t = m.mission.theatre
    end
    if type(t) == "string" then return t end
    return ""
  end

  ---------------------------------------------------------------------------
  -- Game events
  ---------------------------------------------------------------------------
  local function sendEvent(eventName, ...)
    local args = { ... }
    local payload = {
      type = "event",
      event = eventName,
      args = args,
      t = modelTime(),
    }
    sendLine(toJson(payload))
  end

  ---------------------------------------------------------------------------
  -- Players and statistics
  ---------------------------------------------------------------------------
  -- net.get_stat returns an integer; we protect every call.
  local function stat(playerID, id)
    local ok, v = pcall(net.get_stat, playerID, id)
    if ok and type(v) == "number" then return math.floor(v) end
    return 0
  end

  local function info(playerID, attr)
    local ok, v = pcall(net.get_player_info, playerID, attr)
    if ok then return v end
    return nil
  end

  -- Maps a slotID to its aircraft type. Sim.getAvailableSlots returns the
  -- list of available slots with their type; we build the table once per
  -- mission and refresh it periodically (slots can appear).
  local slotTypes = {}

  local function refreshSlotTypes()
    if not (Sim and Sim.getAvailableSlots and Sim.getAvailableCoalitions) then
      return
    end
    local okCo, coalitions = pcall(Sim.getAvailableCoalitions)
    if not okCo or type(coalitions) ~= "table" then return end

    local map = {}
    for coalitionID in pairs(coalitions) do
      local okS, slots = pcall(Sim.getAvailableSlots, coalitionID)
      if okS and type(slots) == "table" then
        for _, slot in ipairs(slots) do
          -- slot = {unitId, type, role, callsign, groupName, country}
          local unitId = slot.unitId or slot[1]
          local unitType = slot.type or slot[2]
          if unitId and unitType then
            map[tostring(unitId)] = unitType
          end
        end
      end
    end
    slotTypes = map
  end

  -- A player's slotID in a multi-seat aircraft is "unitID_seatID"; we keep only
  -- the unitID part to look up the type.
  local function unitTypeForSlot(slot)
    if not slot or slot == "" then return "" end
    if slotTypes[slot] then return slotTypes[slot] end
    local base = slot:match("^(.-)_%d+$")
    if base and slotTypes[base] then return slotTypes[base] end
    return ""
  end

  local function sendPlayers()
    local ok, list = pcall(net.get_player_list)
    if not ok or type(list) ~= "table" then return end

    local players = {}
    for _, id in ipairs(list) do
      local name = info(id, "name")
      if name and name ~= "" then
        local slot = info(id, "slot") or ""
        players[#players + 1] = {
          id = id,
          ucid = info(id, "ucid") or "",
          name = name,
          side = info(id, "side") or 0,
          slot = slot,
          unitType = unitTypeForSlot(slot),
          ping = stat(id, net.PS_PING),
          crashes = stat(id, net.PS_CRASH),
          killsCar = stat(id, net.PS_CAR),
          killsAir = stat(id, net.PS_PLANE),
          killsShip = stat(id, net.PS_SHIP),
          score = stat(id, net.PS_SCORE),
          landings = stat(id, net.PS_LAND),
          ejects = stat(id, net.PS_EJECT),
        }
      end
    end

    sendLine(toJson({ type = "players", players = players }))
  end

  ---------------------------------------------------------------------------
  -- Debrief: reading debrief.log and sending it in chunks
  ---------------------------------------------------------------------------
  -- The file can exceed 1 MB; we send it in base64 chunks so we don't
  -- saturate the connection or block the simulator.
  local CHUNK_BYTES = 32768

  local function readFile(path)
    local f = io.open(path, "rb")
    if not f then return nil end
    local data = f:read("*a")
    f:close()
    return data
  end

  -- Base64 encoding. LuaSocket provides it through mime.b64; socket.base64 is
  -- not part of the API. If neither is present we do not send, rather than send
  -- something the backend cannot decode.
  local function b64(data)
    if mime_ok and mime and mime.b64 then
      local ok, v = pcall(mime.b64, data)
      if ok and v then return v end
    end
    if socket and socket.base64 then
      local ok, v = pcall(socket.base64, data)
      if ok and v then return v end
    end
    return nil
  end

  local function sendDebrief()
    if not (lfs and lfs.writedir) then return end
    local path = lfs.writedir() .. "Logs/debrief.log"
    local data = readFile(path)
    if not data or #data == 0 then
      say("debrief.log not found or empty")
      return
    end

    local chunks = math.ceil(#data / CHUNK_BYTES)
    local transferId = string.format("dbg-%d", os.time())
    local missionName = (Sim and Sim.getMissionName and Sim.getMissionName()) or "?"

    for i = 0, chunks - 1 do
      local part = data:sub(i * CHUNK_BYTES + 1, (i + 1) * CHUNK_BYTES)
      local encoded = b64(part)
      if not encoded then
        say("base64 encoding unavailable, debrief not sent")
        return
      end
      -- A chunk that cannot be written is a failure to report, not something to
      -- announce as sent: that false "debrief sent" is what hid the loss.
      if not sendLine(toJson({
        type = "debrief",
        transferId = transferId,
        chunk = i,
        chunks = chunks,
        size = #data,
        name = missionName,
        data = encoded,
      })) then
        say(string.format("debrief chunk %d/%d could not be sent", i + 1, chunks))
        return
      end
    end
    say(string.format("debrief sent (%d bytes, %d chunks)", #data, chunks))
  end

  --[[
    Command channel (backend → DCS)

    The connection is bidirectional: after sending, we read whatever the backend
    pushed back and execute it. Reading is strictly non-blocking (a zero timeout
    returns "timeout" immediately when there is nothing), because this runs from
    the simulator's frame callback and must never stall a frame.

    The backend sends one JSON object per line:
      {"type":"command","command":"chat","message":"...","from":"Server"}

    We accept a "chat" command and hand it to DCS's chat API. Anything else is
    logged and ignored, so a newer backend cannot break an older hook.
  ]]
  local readBuf = ""

  local function splitLines(s)
    local lines = {}
    local start = 1
    while true do
      local nl = s:find("\n", start, true)
      if not nl then break end
      lines[#lines + 1] = s:sub(start, nl - 1)
      start = nl + 1
    end
    return lines, s:sub(start)
  end

  -- A very small JSON field reader: enough for the flat command objects the
  -- backend sends, without pulling in a parser DCS does not ship.
  local function jsonStringField(s, key)
    local pattern = '"' .. key .. '%s*:%s*"'
    local _, e = s:find(pattern)
    if not e then return nil end
    local i = e + 1
    local out = {}
    while i <= #s do
      local c = s:sub(i, i)
      if c == '"' then break end
      if c == "\\" then
        local n = s:sub(i + 1, i + 1)
        if n == "n" then out[#out + 1] = "\n"
        elseif n == "r" then out[#out + 1] = "\r"
        elseif n == "t" then out[#out + 1] = "\t"
        elseif n == '"' then out[#out + 1] = '"'
        elseif n == "\\" then out[#out + 1] = "\\"
        elseif n == "u" then
          local hex = s:sub(i + 2, i + 5)
          local code = tonumber(hex, 16)
          if code then out[#out + 1] = string.char(code % 256) end
          i = i + 4
        else out[#out + 1] = n end
        i = i + 2
      else
        out[#out + 1] = c
        i = i + 1
      end
    end
    return table.concat(out)
  end

  -- Injects a chat message into DCS. net.send_chat(message, all): without the
  -- second argument it only reaches the sender's own coalition, so the command
  -- channel looked dead for everyone else. `true` broadcasts to all.
  local function injectChat(message, from)
    if not message or message == "" then return end
    local text = message
    if from and from ~= "" then text = "[" .. from .. "] " .. message end
    if net and net.send_chat then
      local ok, err = pcall(net.send_chat, text, true)
      if not ok then say("chat send failed: " .. tostring(err)) end
    end
  end

  local function handleCommand(line)
    if not line:find('"command"', 1, true) then return end
    local cmd = jsonStringField(line, "command")
    if cmd == "chat" then
      injectChat(jsonStringField(line, "message"), jsonStringField(line, "from"))
    else
      say("unknown command from backend: " .. tostring(cmd))
    end
  end

  -- Drains everything the backend has sent since the last call, without ever
  -- blocking.
  local function readCommands()
    if not conn then return end
    if conn.settimeout then conn:settimeout(0) end
    while true do
      local data, err, partial = conn:receive(4096)
      local chunk = data or partial
      if chunk and #chunk > 0 then
        readBuf = readBuf .. chunk
        local lines, rest = splitLines(readBuf)
        readBuf = rest
        for _, line in ipairs(lines) do
          if line ~= "" then handleCommand(line) end
        end
      end
      if not chunk or #chunk == 0 then break end
      -- "timeout" (err set, no partial) means nothing more to read right now.
      if err and err ~= "timeout" then break end
      if data == nil and partial == nil then break end
    end
    if conn.settimeout then conn:settimeout(0.5) end
  end

  ---------------------------------------------------------------------------
  -- Callback table expected by Sim.setUserCallbacks
  ---------------------------------------------------------------------------
  local dcsmanager = {}

  function dcsmanager.onSimulationStart()
    local name = (Sim and Sim.getMissionName and Sim.getMissionName()) or "?"
    refreshSlotTypes()
    -- We forward the mission options: the backend needs them to respect the
    -- mission's visibility rules (fog of war).
    --
    -- The API is DCS.getMissionOptions, NOT Sim.getMissionOptions. The latter
    -- does not exist, so the previous code always failed and sent no options:
    -- the backend then fell back to its restrictive default and only ever
    -- displayed the player's own coalition.
    local options = nil
    local optSource = "none"
    if DCS and DCS.getMissionOptions then
      local ok, opts = pcall(DCS.getMissionOptions)
      if ok and type(opts) == "table" then
        options = opts
        optSource = "DCS.getMissionOptions"
      end
    end
    if not options and Sim and Sim.getMissionOptions then
      -- Kept as a fallback in case a future build exposes it there.
      local ok, opts = pcall(Sim.getMissionOptions)
      if ok and type(opts) == "table" then
        options = opts
        optSource = "Sim.getMissionOptions"
      end
    end

    sendLine(toJson({
      type = "mission",
      phase = "start",
      name = name,
      theatre = missionTheatre(),
      options = options,
    }))
    sendPlayers()
    if options then
      say(string.format("mission started: %s (options from %s)", tostring(name), optSource))
    else
      say("mission started: " .. tostring(name) .. " (no mission options available)")
    end
  end

  function dcsmanager.onSimulationStop()
    -- We send the debrief BEFORE closing the connection: the backend waits
    -- for the file to archive it.
    pcall(sendDebrief)
    sendLine(toJson({ type = "mission", phase = "end" }))
    if conn then pcall(function() conn:close() end) end
    conn = nil
    say("mission ended")
  end

  -- DCS passes up to seven arguments, and the number depends on the event: a
  -- "kill" carries killerPlayerID, killerUnitType, killerSide, victimPlayerID,
  -- victimUnitType, victimSide, weaponName. Declaring fewer parameters silently
  -- dropped the tail, which is why weapon statistics were empty. All of them are
  -- forwarded; the backend keeps them as-is.
  function dcsmanager.onGameEvent(eventName, arg1, arg2, arg3, arg4, arg5, arg6, arg7)
    local args = {}
    for _, a in ipairs({ arg1, arg2, arg3, arg4, arg5, arg6, arg7 }) do
      if a ~= nil then args[#args + 1] = a end
    end
    sendLine(toJson({
      type = "event",
      event = eventName,
      args = args,
      t = modelTime(),
    }))

    -- Diagnostic: log each distinct event name once per session, so the DCS log
    -- shows exactly which game events the hook actually receives. In single
    -- player DCS delivers fewer events than in multiplayer, and without this the
    -- manager just looks like it is dropping them.
    eventNamesSeen = eventNamesSeen or {}
    if not eventNamesSeen[eventName] then
      eventNamesSeen[eventName] = true
      say("game event: " .. tostring(eventName))
    end

    -- A slot change or a connection changes the players' state:
    -- we refresh right away so the UI stays consistent.
    if eventName == "change_slot" or eventName == "connect"
       or eventName == "disconnect" or eventName == "mission_end" then
      sendPlayers()
    end
  end

  -- DCS passes a numeric playerID as `from`. The backend decodes `from` as a
  -- string, so sending the number made the whole JSON line fail to unmarshal and
  -- every chat message was silently dropped. Resolve the player's name; fall back
  -- to the id as text so a message is never lost.
  function dcsmanager.onChatMessage(message, from)
    local sender = ""
    if from ~= nil then
      sender = info(from, "name") or ""
      if sender == "" then sender = tostring(from) end
    end
    sendLine(toJson({ type = "chat", from = sender, message = message or "" }))
  end

  function dcsmanager.onPlayerConnect(id) sendPlayers() end
  function dcsmanager.onPlayerDisconnect(id, code) sendPlayers() end
  function dcsmanager.onPlayerChangeSlot(id) sendPlayers() end

  -- Periodic refresh of statistics, via the simulator timer.
  local nextPlayersAt, nextSlotsAt, nextCommandAt
  -- How often we look for a command from the backend. Short enough to feel
  -- responsive, long enough that the socket read (which never blocks anyway)
  -- stays negligible against the frame budget.
  local commandInterval = 0.25

  function dcsmanager.onSimulationFrame()
    local t = modelTime()
    if not nextSlotsAt then nextSlotsAt = t + 30.0 end
    if t >= nextSlotsAt then
      refreshSlotTypes()
      nextSlotsAt = t + 30.0
    end
    if not nextPlayersAt then nextPlayersAt = t + playersInterval end
    if t >= nextPlayersAt then
      sendPlayers()
      nextPlayersAt = t + playersInterval
    end
    if not nextCommandAt then nextCommandAt = t + commandInterval end
    if t >= nextCommandAt then
      pcall(readCommands)
      nextCommandAt = t + commandInterval
    end
  end

  Sim.setUserCallbacks(dcsmanager)
  say("hooks loaded")
end
-- <<< DCSMANAGER-END <<<
