--[[
  DCS Mission Manager — Export.lua
  ------------------------------------------------------------------
  Envoie la position du joueur au backend en UDP/JSON, une fois par
  seconde, sans impacter les performances du simulateur.

  ⚠️  NE PAS REMPLACER un Export.lua existant (Tacview, SRS, DCS-BIOS…).
      Ajoute plutôt le bloc `do ... end` ci-dessous À LA FIN de ton
      Export.lua existant, ou copie ce fichier si tu n'en as pas.

  Prérequis : Config/dcsmm.cfg présent dans Saved Games\DCS\Config\
]]

do
  -- Le logger `log.*` est disponible dans tous les états Lua ; `net.*` et
  -- `lfs.*` ne le sont pas forcément selon le contexte, d'où les gardes.
  local function say(msg)
    if log and log.write then
      pcall(log.write, "DCSMM", log.INFO or 0, msg)
    end
  end

  local host, udpPort, interval, enabled = "127.0.0.1", 7778, 1.0, true

  -- Chargement défensif de la configuration utilisateur.
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
      if env.dcsmm_enabled ~= nil then enabled = env.dcsmm_enabled end
    end
  end

  local socket_ok, socket = pcall(require, "socket")
  if not socket_ok then
    say("LuaSocket introuvable, export désactivé")
    return
  end

  local conn

  local function connect()
    conn = socket.udp()
    conn:setpeername(host, udpPort)
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

  local function sendOwnship()
    local data = LoGetSelfData and LoGetSelfData() or nil
    if not data or not data.LatLongAlt then
      return
    end

    local name = (LoGetPilotName and LoGetPilotName()) or "Player"
    -- Dans l'API Export, `SelfData.Name` est le type d'appareil (ex. F-16C_50).
    local unitType = data.Name or "unknown"
    local coalition = data.Coalition or "unknown"

    local payload = string.format(
      '{"type":"ownship","name":"%s","unitType":"%s","coalition":"%s",' ..
      '"lat":%.6f,"lng":%.6f,"alt":%.2f,"heading":%.2f,"modelTime":%.2f}',
      jsonEscape(name),
      jsonEscape(unitType),
      jsonEscape(coalition),
      data.LatLongAlt.Lat,
      data.LatLongAlt.Long,
      data.LatLongAlt.Alt,
      data.Heading or 0,
      (LoGetModelTime and LoGetModelTime()) or 0
    )

    if conn then
      pcall(function() conn:send(payload) end)
    end
  end

  function LuaExportStart()
    if not enabled then return end
    pcall(connect)
    say("export des positions activé (" .. host .. ":" .. tostring(udpPort) .. ")")
  end

  function LuaExportStop()
    if conn then pcall(function() conn:close() end) end
  end

  -- Appelé par le timer du simulateur ; on planifie le prochain envoi afin
  -- de ne jamais bloquer une frame.
  function LuaExportActivityNextEvent(t)
    if not enabled then return t end
    if not conn then pcall(connect) end
    pcall(sendOwnship)
    return t + interval
  end
end
