--[[
  DCS Mission Manager — Export.lua
  ------------------------------------------------------------------
  Envoie au backend, en UDP/JSON :
    - la position du joueur (message "ownship") ;
    - la liste des objets du monde (message "world"), filtrée pour
      ne garder que les unités utiles à la live map.

  Tout est échantillonné à intervalle régulier via
  LuaExportActivityNextEvent, sans jamais bloquer une frame.

  ⚠️  NE PAS REMPLACER un Export.lua existant (Tacview, SRS, DCS-BIOS…).
      Ajoute plutôt le bloc `do ... end` ci-dessous À LA FIN de ton
      Export.lua existant, ou copie ce fichier si tu n'en as pas.

  Prérequis : Config/dcsmm.cfg présent dans Saved Games\DCS\Config\

  Options (dcsmm.cfg) :
    dcsmm_send_interval   intervalle d'envoi du joueur, en secondes (défaut 1.0)
    dcsmm_world_interval  intervalle d'envoi du monde, en secondes (défaut 2.0)
    dcsmm_world_enabled   activer/désactiver l'export du monde (défaut true)
    dcsmm_world_radius    rayon max en km autour du joueur (0 = pas de limite)
    dcsmm_max_objects     nombre max d'objets par message (défaut 800)
    dcsmm_coalitions      liste des coalitions à inclure (défaut {"blue","red"})
]]

do
  ---------------------------------------------------------------------------
  -- Journalisation défensive
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
    say("LuaSocket introuvable, export désactivé")
    return
  end

  local conn
  local nextWorldAt

  local function connect()
    conn = socket.udp()
    conn:setpayloadsize(65507)          -- maximise la taille des datagrammes
    conn:setpeername(host, udpPort)
  end

  local function send(payload)
    if not conn then pcall(connect) end
    if conn then pcall(function() conn:send(payload) end) end
  end

  ---------------------------------------------------------------------------
  -- Encodage JSON minimal (les chaînes DCS ne contiennent pas de caractères
  -- exotiques, mais on échappe tout de même les caractères sensibles).
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
  -- Position du joueur (+ télémétrie avancée si autorisée par le serveur)
  ---------------------------------------------------------------------------
  -- L'export "ownship" (vitesse, G, incidence…) dépend d'une option serveur.
  -- On teste sa disponibilité une fois et on l'utilise si possible.
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

  -- Accès défensif aux fonctions d'export (globales ou dans Export.).
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

    local payload = {
      type = "ownship",
      name = name,
      unitType = data.Name or "unknown",
      coalition = data.Coalition or "unknown",
      lat = ownshipLat,
      lng = ownshipLng,
      alt = data.LatLongAlt.Alt or 0,
      heading = data.Heading or 0,
      modelTime = (getTime and getTime()) or 0,
    }

    -- Télémétrie avancée, uniquement si le serveur l'autorise.
    if checkOwnshipExport() then
      payload.tas = tryCall(exportFn("LoGetTrueAirSpeed"), 0)
      payload.ias = tryCall(exportFn("LoGetIndicatedAirSpeed"), 0)
      payload.mach = tryCall(exportFn("LoGetMachNumber"), 0)
      payload.aoa = tryCall(exportFn("LoGetAngleOfAttack"), 0)
      payload.altAgl = tryCall(exportFn("LoGetAltitudeAboveGroundLevel"), 0)

      local accel = exportFn("LoGetAccelerationUnits")
      if accel then
        local ok, a = pcall(accel)
        if ok and type(a) == "table" and type(a.y) == "number" then
          payload.g = a.y
        end
      end
    end

    send(toJson(payload))
  end

  ---------------------------------------------------------------------------
  -- Objets du monde
  ---------------------------------------------------------------------------
  -- Distance approximative en km (suffisante pour un filtre de rayon).
  local function approxDistanceKm(lat1, lng1, lat2, lng2)
    local dLat = (lat2 - lat1) * 111.0
    local dLng = (lng2 - lng1) * 111.0 * math.cos(math.rad(lat1))
    return math.sqrt(dLat * dLat + dLng * dLng)
  end

  local function sendWorld()
    -- Selon l'état Lua, les fonctions d'export sont globales (LoGet…) ou
    -- regroupées dans le namespace Export. ; on résout les deux.
    local getWorld = LoGetWorldObjects or (Export and Export.LoGetWorldObjects)
    local isAllowed = LoIsObjectExportAllowed or (Export and Export.LoIsObjectExportAllowed)
    if not getWorld then return end

    -- En multijoueur, l'export d'objets dépend d'une option serveur.
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
    say("export activé (" .. host .. ":" .. tostring(udpPort) .. ")")
  end

  function LuaExportStop()
    if conn then pcall(function() conn:close() end) end
  end

  function LuaExportActivityNextEvent(t)
    if not enabled then return t end
    pcall(sendOwnship)

    if worldEnabled then
      -- On décale le premier envoi du monde pour ne pas tout envoyer d'un coup.
      if not nextWorldAt then nextWorldAt = t + 0.5 end
      if t >= nextWorldAt then
        pcall(sendWorld)
        nextWorldAt = t + worldInterval
      end
    end

    return t + interval
  end
end
