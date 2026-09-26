--[[
  DCS Mission Manager — Export.lua
  ------------------------------------------------------------------
  Sends to the backend, over UDP/JSON:
    - the player position ("ownship" message);
    - the list of world objects ("world" message), filtered to
      keep only the units useful to the live map.

  Everything is sampled at a regular interval via
  LuaExportActivityNextEvent, without ever blocking a frame.

  Installation: `dcsmm install-lua` merges this block into your existing
  Export.lua (Tacview, SRS, DCS-BIOS...) by placing it between the DCSMM
  markers. Do not remove the markers: they are used for updating and
  uninstalling (`dcsmm uninstall-lua`).

  Prerequisite: Config/dcsmm.cfg present in Saved Games\DCS\Config\

  Options (dcsmm.cfg):
    dcsmm_send_interval   player send interval, in seconds (default 1.0)
    dcsmm_world_interval  world send interval, in seconds (default 2.0)
    dcsmm_world_enabled   enable/disable the world export (default true)
    dcsmm_world_radius    max radius in km around the player (0 = no limit)
    dcsmm_max_objects     max number of objects per message (default 800)
    dcsmm_coalitions      list of coalitions to include (default {"blue","red"})
]]

-- >>> DCSMM-BEGIN (managed block — do not edit by hand) >>>
do
  ---------------------------------------------------------------------------
  -- Defensive logging
  ---------------------------------------------------------------------------
  local function say(msg)
    if log and log.write then
      pcall(log.write, "DCSMM", log.INFO or 0, msg)
    end
  end

  ---------------------------------------------------------------------------
  -- Configuration
  ---------------------------------------------------------------------------
  local host, udpPort = "127.0.0.1", 7778
  local interval, worldInterval = 1.0, 2.0
  local enabled, worldEnabled = true, true
  local worldRadiusKm, maxObjects = 0, 800
  local coalitions = { blue = true, red = true }
  local ownshipLat, ownshipLng = nil, nil

  if lfs and lfs.writedir then
    local cfgPath = lfs.writedir() .. "Config/dcsmm.cfg"
    local ok, chunk = pcall(loadfile, cfgPath)
    if ok and chunk then
      local env = {}
      setmetatable(env, { __index = _G })
      setfenv(chunk, env)
      pcall(chunk)

      host = env.dcsmm_host or host
      udpPort = env.dcsmm_udp_port or udpPort
      interval = env.dcsmm_send_interval or interval
      worldInterval = env.dcsmm_world_interval or worldInterval
      worldRadiusKm = env.dcsmm_world_radius or worldRadiusKm
      maxObjects = env.dcsmm_max_objects or maxObjects
      if env.dcsmm_enabled ~= nil then enabled = env.dcsmm_enabled end
      if env.dcsmm_world_enabled ~= nil then worldEnabled = env.dcsmm_world_enabled end
      if type(env.dcsmm_coalitions) == "table" then
        coalitions = {}
        for _, c in ipairs(env.dcsmm_coalitions) do coalitions[c] = true end
      end
    end
  end

  local socket_ok, socket = pcall(require, "socket")
  if not socket_ok then
    say("LuaSocket not found, export disabled")
    return
  end

  local conn
  local nextWorldAt

  local function connect()
    conn = socket.udp()
    conn:setpayloadsize(65507)          -- maximize datagram size
    conn:setpeername(host, udpPort)
  end

  local function send(payload)
    if not conn then pcall(connect) end
    if conn then pcall(function() conn:send(payload) end) end
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

    local name = (getPilot and getPilot()) or "Player"
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
    send('{"type":"world","count":' .. count .. ',"units":[' ..
      table.concat(parts, ",") .. ']}')
  end

  ---------------------------------------------------------------------------
  -- Callbacks Export
  ---------------------------------------------------------------------------
  function LuaExportStart()
    if not enabled then return end
    pcall(connect)
    say("export enabled (" .. host .. ":" .. tostring(udpPort) .. ")")
  end

  function LuaExportStop()
    if conn then pcall(function() conn:close() end) end
  end

  function LuaExportActivityNextEvent(t)
    if not enabled then return t end
    pcall(sendOwnship)

    if worldEnabled then
      -- We delay the first world send so we don't send everything at once.
      if not nextWorldAt then nextWorldAt = t + 0.5 end
      if t >= nextWorldAt then
        pcall(sendWorld)
        nextWorldAt = t + worldInterval
      end
    end

    return t + interval
  end
end
-- <<< DCSMM-END <<<
