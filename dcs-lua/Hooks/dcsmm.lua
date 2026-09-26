--[[
  DCS Mission Manager — Hooks/dcsmm.lua
  ------------------------------------------------------------------
  Sends to the backend, over TCP/JSON (one JSON line per message):
    - game events (kill, crash, eject, takeoff, landing...);
    - the list of connected players with their statistics;
    - the chat;
    - the mission start and end.

  Place this in: Saved Games\DCS\Scripts\Hooks\dcsmm.lua

  ⚠️  DCS loads ALL .lua files from Hooks/ and sorts them by name.
      This file is additive: it does not overwrite any other hook.

  Prerequisite: Config/dcsmm.cfg present in Saved Games\DCS\Config\

  Options (dcsmm.cfg):
    dcsmm_host            backend address (default 127.0.0.1)
    dcsmm_tcp_port        backend TCP port (default 7779)
    dcsmm_enabled         enable/disable (default true)
    dcsmm_players_interval players send interval in seconds (default 5.0)
]]

-- >>> DCSMM-BEGIN (managed block — do not edit by hand) >>>
do
  local function say(msg)
    if net and net.log then net.log("DCSMM: " .. tostring(msg)) end
  end
  ---------------------------------------------------------------------------
  -- Configuration
  ---------------------------------------------------------------------------
  local host, tcpPort = "127.0.0.1", 7779
  local enabled = true
  local playersInterval = 5.0

  if lfs and lfs.writedir then
    local cfgPath = lfs.writedir() .. "Config/dcsmm.cfg"
    local ok, chunk = pcall(loadfile, cfgPath)
    if ok and chunk then
      local env = {}
      setmetatable(env, { __index = _G })
      setfenv(chunk, env)
      pcall(chunk)
      host = env.dcsmm_host or host
      tcpPort = env.dcsmm_tcp_port or tcpPort
      playersInterval = env.dcsmm_players_interval or playersInterval
      if env.dcsmm_enabled ~= nil then enabled = env.dcsmm_enabled end
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

  local function sendLine(line)
    if not conn and not connect() then
      return false
    end
    local ok, err = conn:send(line .. "\n")
    if not ok then
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

  ---------------------------------------------------------------------------
  -- Game events
  ---------------------------------------------------------------------------
  local function sendEvent(eventName, ...)
    local args = { ... }
    local payload = {
      type = "event",
      event = eventName,
      args = args,
      t = (LoGetModelTime and LoGetModelTime()) or 0,
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

  -- Base64 encoding (LuaSocket provides it; otherwise we don't send).
  local function b64(data)
    if socket and socket.base64 then
      local ok, v = pcall(socket.base64, data)
      if ok then return v end
    end
    if mime and mime.b64 then
      local ok, v = pcall(mime.b64, data)
      if ok then return v end
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
      sendLine(toJson({
        type = "debrief",
        transferId = transferId,
        chunk = i,
        chunks = chunks,
        size = #data,
        name = missionName,
        data = encoded,
      }))
    end
    say(string.format("debrief sent (%d bytes, %d chunks)", #data, chunks))
  end

  ---------------------------------------------------------------------------
  -- Callback table expected by Sim.setUserCallbacks
  ---------------------------------------------------------------------------
  local dcsmm = {}

  function dcsmm.onSimulationStart()
    local name = (Sim and Sim.getMissionName and Sim.getMissionName()) or "?"
    refreshSlotTypes()
    -- We forward the mission options: the backend needs them to
    -- respect the mission's visibility rules (fog of war).
    local options = nil
    if Sim and Sim.getMissionOptions then
      local ok, opts = pcall(Sim.getMissionOptions)
      if ok and type(opts) == "table" then options = opts end
    end
    sendLine(toJson({
      type = "mission",
      phase = "start",
      name = name,
      options = options,
    }))
    sendPlayers()
    say("mission started: " .. tostring(name))
  end

  function dcsmm.onSimulationStop()
    -- We send the debrief BEFORE closing the connection: the backend waits
    -- for the file to archive it.
    pcall(sendDebrief)
    sendLine(toJson({ type = "mission", phase = "end" }))
    if conn then pcall(function() conn:close() end) end
    conn = nil
    say("mission ended")
  end

  function dcsmm.onGameEvent(eventName, arg1, arg2, arg3, arg4)
    -- We forward all non-nil arguments; the backend stores them as-is.
    local args = {}
    for _, a in ipairs({ arg1, arg2, arg3, arg4 }) do
      if a ~= nil then args[#args + 1] = a end
    end
    sendLine(toJson({
      type = "event",
      event = eventName,
      args = args,
      t = (LoGetModelTime and LoGetModelTime()) or 0,
    }))

    -- A slot change or a connection changes the players' state:
    -- we refresh right away so the UI stays consistent.
    if eventName == "change_slot" or eventName == "connect"
       or eventName == "disconnect" or eventName == "mission_end" then
      sendPlayers()
    end
  end

  function dcsmm.onChatMessage(message, from)
    sendLine(toJson({ type = "chat", from = from or "", message = message or "" }))
  end

  function dcsmm.onPlayerConnect(id) sendPlayers() end
  function dcsmm.onPlayerDisconnect(id, code) sendPlayers() end
  function dcsmm.onPlayerChangeSlot(id) sendPlayers() end

  -- Periodic refresh of statistics, via the simulator timer.
  local nextPlayersAt, nextSlotsAt
  function dcsmm.onSimulationFrame()
    local t = (LoGetModelTime and LoGetModelTime()) or 0
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
  end

  Sim.setUserCallbacks(dcsmm)
  say("hooks loaded")
end
-- <<< DCSMM-END <<<
