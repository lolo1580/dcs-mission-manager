--[[
  DCS Manager — Export.lua
  ------------------------------------------------------------------
  Sends to the backend, over UDP/JSON:
    - the player position ("ownship" message);
    - the list of world objects ("world" message), filtered to
      keep only the units useful to the analysis.

  Everything is sampled at a regular interval via
  LuaExportActivityNextEvent, without ever blocking a frame.

  Installation: `dcsmanager install-lua` merges this block into your existing
  Export.lua (Tacview, SRS, DCS-BIOS...) by placing it between the DCSMANAGER
  markers. Do not remove the markers: they are used for updating and
  uninstalling (`dcsmanager uninstall-lua`).

  Prerequisite: Config/dcsmanager.cfg present in Saved Games\DCS\Config\

  Options (dcsmanager.cfg):
    dcsmanager_send_interval   player send interval, in seconds (default 1.0)
    dcsmanager_world_interval  world send interval, in seconds (default 2.0)
    dcsmanager_world_enabled   enable/disable the world export (default true)
    dcsmanager_world_radius    max radius in km around the player (0 = no limit)
    dcsmanager_max_objects     max number of objects per message (default 800)
    dcsmanager_coalitions      list of coalitions to include (default {"blue","red"})
]]

-- >>> DCSMANAGER-BEGIN (managed block — do not edit by hand) >>>
do
  ---------------------------------------------------------------------------
  -- Defensive logging
  ---------------------------------------------------------------------------
  local function say(msg)
    if log and log.write then
      pcall(log.write, "DCSMANAGER", log.INFO or 0, msg)
    end
  end

  ---------------------------------------------------------------------------
  -- Chain the export callbacks instead of replacing them
  ---------------------------------------------------------------------------
  -- DCS calls the LAST defined global of each callback, so a block that defines
  -- LuaExportStart/LuaExportStop/LuaExportActivityNextEvent without calling the
  -- previous one silently disables every tool loaded before it. DCS-BIOS loads
  -- first (dofile at the top of Export.lua) and relies on LuaExportStart to open
  -- its sockets; overriding it here left DCS-BIOS never started — the panels
  -- looked dead and the link reported "not connected". Capturing and calling the
  -- previous functions is the same pattern DCS-BIOS itself uses.
  local prevStart = LuaExportStart
  local prevStop = LuaExportStop
  local prevActivity = LuaExportActivityNextEvent
  local prevBefore = LuaExportBeforeNextFrame

  ---------------------------------------------------------------------------
  -- Configuration
  ---------------------------------------------------------------------------
  local host, udpPort = "127.0.0.1", 7776
  local interval, worldInterval = 1.0, 2.0
  local enabled, worldEnabled = true, true
  local worldRadiusKm, maxObjects = 0, 800
  local coalitions = { blue = true, red = true }
  local ownshipLat, ownshipLng = nil, nil

  if lfs and lfs.writedir then
    local cfgPath = lfs.writedir() .. "Config/dcsmanager.cfg"
    local ok, chunk = pcall(loadfile, cfgPath)
    if ok and chunk then
      local env = {}
      setmetatable(env, { __index = _G })
      setfenv(chunk, env)
      pcall(chunk)

      host = env.dcsmanager_host or host
      udpPort = env.dcsmanager_udp_port or udpPort
      interval = env.dcsmanager_send_interval or interval
      worldInterval = env.dcsmanager_world_interval or worldInterval
      worldRadiusKm = env.dcsmanager_world_radius or worldRadiusKm
      maxObjects = env.dcsmanager_max_objects or maxObjects
      if env.dcsmanager_enabled ~= nil then enabled = env.dcsmanager_enabled end
      if env.dcsmanager_world_enabled ~= nil then worldEnabled = env.dcsmanager_world_enabled end
      if type(env.dcsmanager_coalitions) == "table" then
        coalitions = {}
        for _, c in ipairs(env.dcsmanager_coalitions) do coalitions[c] = true end
      end
    end
  end

  -- A zero or invalid interval must never schedule the next callback at the
  -- current model time: DCS may then stop calling it after the first packet.
  if type(interval) ~= "number" or interval < 0.05 then interval = 1.0 end
  if type(worldInterval) ~= "number" or worldInterval < 0.05 then worldInterval = 2.0 end

  local socket_ok, socket = pcall(require, "socket")
  if not socket_ok then
    say("LuaSocket not found, export disabled")
    return
  end

  local conn
  local nextSendAt
  local nextWorldAt
  local panelCommands
  if lfs and lfs.writedir then
    local ok, factory = pcall(dofile, lfs.writedir() .. "Scripts/DCSManager/PanelCommands.lua")
    if ok and type(factory) == "function" then
      panelCommands = factory(socket, say)
    else
      say("panel command plugin missing or invalid; reinstall Lua scripts")
    end
  end

  -- Largest UDP payload we will build. LuaSocket's send() fails outright when a
  -- datagram exceeds the socket limit, and DCS's build has no setpayloadsize to
  -- raise it, so the default applies: world messages are split to fit.
  local MAX_DATAGRAM = 7000

  local function connect()
    conn = socket.udp()
    -- setpayloadsize does NOT exist in the LuaSocket shipped with DCS. Calling it
    -- raised an error that aborted connect() before setpeername, leaving the
    -- socket with no destination: every send then failed silently, which is why
    -- the map received nothing and no error ever reached the log.
    if conn and conn.setpayloadsize then
      pcall(conn.setpayloadsize, conn, 65507)
    end
    local ok, err = pcall(conn.setpeername, conn, host, udpPort)
    if not ok then
      conn = nil
      return false, tostring(err)
    end
    return true
  end

  local sendFailures = 0
  local function send(payload)
    if not conn then
      local ok = connect()
      if not ok then
        if sendFailures == 0 then say("UDP connect failed (" .. host .. ":" .. tostring(udpPort) .. ")") end
        sendFailures = sendFailures + 1
        return false
      end
    end
    local ok, err = pcall(conn.send, conn, payload)
    if not ok then
      -- Never swallow this again: silence was the real bug.
      if sendFailures < 3 then say("UDP send failed: " .. tostring(err)) end
      sendFailures = sendFailures + 1
      conn = nil -- force a reconnect on the next attempt
      return false
    end
    return true
  end

  ---------------------------------------------------------------------------
  -- Minimal JSON encoding (DCS strings don't contain exotic characters,
  -- but we still escape sensitive characters).
  ---------------------------------------------------------------------------
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

  ---------------------------------------------------------------------------
  -- Player position (+ advanced telemetry if allowed by the server)
  ---------------------------------------------------------------------------
  -- The "ownship" export (speed, G, AoA...) depends on a server option.
  -- We test its availability once and use it if possible.
  local ownshipExportAllowed = nil
  local function checkOwnshipExport()
    if ownshipExportAllowed ~= nil then return ownshipExportAllowed end
    local isAllowed = LoIsOwnshipExportAllowed or (Export and Export.LoIsOwnshipExportAllowed)
    if not isAllowed then
      ownshipExportAllowed = false
      return false
    end
    local ok, res = pcall(isAllowed)
    ownshipExportAllowed = (ok and res == true)
    return ownshipExportAllowed
  end

  -- Defensive access to export functions (global or inside Export.).
  local function exportFn(name)
    return _G[name] or (Export and Export[name])
  end

  local function tryCall(fn, fallback)
    if not fn then return fallback end
    local ok, v = pcall(fn)
    if ok and type(v) == "number" then return v end
    return fallback
  end

  local function sendOwnship()
    local getSelf = exportFn("LoGetSelfData")
    local getPilot = exportFn("LoGetPilotName")
    local getTime = exportFn("LoGetModelTime")

    local data = getSelf and getSelf() or nil
    if not data or not data.LatLongAlt then return end

    -- Name the ownship so the app can identify the player. It used to send only
    -- the aircraft type, which meant the unit list showed an unnamed aircraft.
    local okName, pilot = pcall(function()
      return getPilot and getPilot()
    end)
    local name = (okName and pilot) or "Player"
    ownshipLat = data.LatLongAlt.Lat
    ownshipLng = data.LatLongAlt.Long

    -- Advanced telemetry, only if the server allows it. We build the JSON
    -- payload directly: optional fields are only added if they are actually
    -- available.
    local extra = ""
    if checkOwnshipExport() then
      local tas = tryCall(exportFn("LoGetTrueAirSpeed"), 0)
      local ias = tryCall(exportFn("LoGetIndicatedAirSpeed"), 0)
      local mach = tryCall(exportFn("LoGetMachNumber"), 0)
      local aoa = tryCall(exportFn("LoGetAngleOfAttack"), 0)
      local altAgl = tryCall(exportFn("LoGetAltitudeAboveGroundLevel"), 0)
      extra = string.format(
        ',"tas":%.2f,"ias":%.2f,"mach":%.3f,"aoa":%.4f,"altAgl":%.1f',
        tas, ias, mach, aoa, altAgl)

      local accel = exportFn("LoGetAccelerationUnits")
      if accel then
        local ok, a = pcall(accel)
        if ok and type(a) == "table" and type(a.y) == "number" then
          extra = extra .. string.format(',"g":%.2f', a.y)
        end
      end
    end

    send(string.format(
      '{"type":"ownship","name":"%s","unitType":"%s","coalition":"%s",' ..
      '"lat":%.6f,"lng":%.6f,"alt":%.2f,"heading":%.2f,"modelTime":%.2f%s}',
      jsonEscape(name),
      jsonEscape(data.Name or "unknown"),
      jsonEscape(data.Coalition or "unknown"),
      ownshipLat, ownshipLng,
      data.LatLongAlt.Alt or 0,
      data.Heading or 0,
      (getTime and getTime()) or 0,
      extra
    ))
  end

  ---------------------------------------------------------------------------
  -- World objects
  ---------------------------------------------------------------------------
  -- Approximate distance in km (good enough for a radius filter).
  local function approxDistanceKm(lat1, lng1, lat2, lng2)
    local dLat = (lat2 - lat1) * 111.0
    local dLng = (lng2 - lng1) * 111.0 * math.cos(math.rad(lat1))
    return math.sqrt(dLat * dLat + dLng * dLng)
  end

  local function sendWorld()
    -- Depending on the Lua state, the export functions are global (LoGet...) or
    -- grouped in the Export. namespace; we resolve both.
    local getWorld = LoGetWorldObjects or (Export and Export.LoGetWorldObjects)
    local isAllowed = LoIsObjectExportAllowed or (Export and Export.LoIsObjectExportAllowed)
    if not getWorld then return end

    -- In multiplayer, object export depends on a server option.
    if isAllowed then
      local ok, res = pcall(isAllowed)
      if ok and res == false then return end
    end

    local ok2, objects = pcall(getWorld)
    if not ok2 or type(objects) ~= "table" then return end

    local parts, count = {}, 0
    for id, o in pairs(objects) do
      if count >= maxObjects then break end
      if type(o) == "table" and o.LatLongAlt then
        local co = o.Coalition or "neutral"
        local include = coalitions[co] == true
        if include then
          local lat, lng = o.LatLongAlt.Lat, o.LatLongAlt.Long
          if ownshipLat and worldRadiusKm > 0 then
            include = approxDistanceKm(ownshipLat, ownshipLng, lat, lng) <= worldRadiusKm
          end
          if include then
            count = count + 1
            parts[count] = string.format(
              '{"id":"%s","type":"%s","coalition":"%s","country":"%s",' ..
              '"lat":%.6f,"lng":%.6f,"alt":%.1f,"heading":%.1f}',
              jsonEscape(tostring(id)),
              jsonEscape(o.TypeName or o.Name or "unknown"),
              jsonEscape(co),
              jsonEscape(o.Country or ""),
              lat, lng,
              o.LatLongAlt.Alt or 0,
              o.Heading or 0
            )
          end
        end
      end
    end

    if count == 0 then return end

    -- Send in batches whose JSON stays under the datagram limit: a single large
    -- message (hundreds of units) used to fail at the socket level and vanish.
    local batch, batchBytes = {}, 0
    local function flush()
      if #batch == 0 then return end
      send('{"type":"world","count":' .. #batch .. ',"units":[' .. table.concat(batch, ",") .. ']}')
      batch, batchBytes = {}, 0
    end

    for i = 1, count do
      local part = parts[i]
      if batchBytes + #part > MAX_DATAGRAM then flush() end
      batch[#batch + 1] = part
      batchBytes = batchBytes + #part + 1
    end
    flush()
  end

  ---------------------------------------------------------------------------
  -- Callbacks Export
  ---------------------------------------------------------------------------
  -- Each callback runs the previous tool's (DCS-BIOS, Tacview…) first, then its
  -- own work, so loading after them does not disable them.
  function LuaExportStart()
    if prevStart then pcall(prevStart) end
    nextSendAt, nextWorldAt = nil, nil
    ownshipExportAllowed = nil
    if not enabled then return end
    if panelCommands then panelCommands.start() end
    local ok = connect()
    if ok then
      say("export enabled (" .. host .. ":" .. tostring(udpPort) .. ")")
    else
      say("export FAILED to open the UDP socket (" .. host .. ":" .. tostring(udpPort) .. ")")
    end
  end

  function LuaExportStop()
    if panelCommands then panelCommands.stop() end
    if conn then pcall(function() conn:close() end) end
    if prevStop then pcall(prevStop) end
  end

  function LuaExportBeforeNextFrame()
    if prevBefore then pcall(prevBefore) end
    if enabled and panelCommands then
      local ok, err = pcall(panelCommands.frame)
      if not ok then say("panel command frame failed: " .. tostring(err)); panelCommands.stop() end
    end
  end

  function LuaExportActivityNextEvent(t)
    -- Let the previous tool schedule its own next event and keep the earliest,
    -- so chaining never delays its sampling. Returning t itself made DCS call
    -- this export only once; the heartbeat then expired after the backend TTL.
    local nextT
    if prevActivity then
      local ok, v = pcall(prevActivity, t)
      if ok and type(v) == "number" and v > t and v < math.huge then nextT = v end
    end

    if enabled and (not nextSendAt or t >= nextSendAt) then
      -- Activity can continue without an ownship (spectator) or exportable
      -- world objects (server restrictions). Keep the backend informed without
      -- creating a fake unit on the map.
      send('{"type":"heartbeat"}')
      pcall(sendOwnship)

      if worldEnabled then
        -- We delay the first world send so we don't send everything at once.
        if not nextWorldAt then nextWorldAt = t + 0.5 end
        if t >= nextWorldAt then
          pcall(sendWorld)
          nextWorldAt = t + worldInterval
        end
      end

      nextSendAt = t + interval
    end
    if enabled and (not nextT or nextSendAt < nextT) then
      nextT = nextSendAt
    end

    -- DCS requires a future model time. Keep the callback alive even when our
    -- export is disabled and the previous callback has no valid deadline.
    return nextT or (t + 1.0)
  end
end
-- <<< DCSMANAGER-END <<<
