--[[
  DCS Mission Manager — Hooks/dcsmm.lua
  ------------------------------------------------------------------
  Envoie au backend, en TCP/JSON (une ligne JSON par message) :
    - les événements de jeu (kill, crash, eject, takeoff, landing…) ;
    - la liste des joueurs connectés avec leurs statistiques ;
    - le chat ;
    - le début et la fin de mission.

  À placer dans : Saved Games\DCS\Scripts\Hooks\dcsmm.lua

  ⚠️  DCS charge TOUS les fichiers .lua de Hooks/ et les trie par nom.
      Ce fichier est additif : il n'écrase aucun autre hook.

  Prérequis : Config/dcsmm.cfg présent dans Saved Games\DCS\Config\

  Options (dcsmm.cfg) :
    dcsmm_host            adresse du backend (défaut 127.0.0.1)
    dcsmm_tcp_port        port TCP du backend (défaut 7779)
    dcsmm_enabled         activer/désactiver (défaut true)
    dcsmm_players_interval intervalle d'envoi des joueurs en secondes (défaut 5.0)
]]

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
    say("hooks désactivés par la configuration")
    return
  end

  local socket_ok, socket = pcall(require, "socket")
  if not socket_ok then
    say("LuaSocket introuvable, hooks désactivés")
    return
  end

  ---------------------------------------------------------------------------
  -- Connexion TCP persistante (reconnexion automatique)
  ---------------------------------------------------------------------------
  local conn

  local function connect()
    local c, err = socket.tcp()
    if not c then return false end
    c:settimeout(0.5)               -- ne jamais bloquer le simulateur
    local okConn, cerr = c:connect(host, tcpPort)
    if not okConn and cerr ~= "already connected" then
      c:close()
      return false
    end
    c:setoption("tcp-nodelay", true)
    conn = c
    say("connecté au backend " .. host .. ":" .. tostring(tcpPort))
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

  -- Sérialise une valeur Lua en JSON (types simples, listes, tables).
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
      -- Détection liste vs objet.
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
  -- Événements de jeu
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
  -- Joueurs et statistiques
  ---------------------------------------------------------------------------
  -- net.get_stat renvoie un entier ; on protège chaque appel.
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

  -- Associe un slotID à son type d'appareil. Sim.getAvailableSlots renvoie la
  -- liste des slots disponibles avec leur type ; on construit la table une fois
  -- par mission et on la rafraîchit périodiquement (des slots peuvent apparaître).
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

  -- Le slotID d'un joueur en multi-siège vaut "unitID_seatID" ; on ne garde que
  -- la partie unitID pour retrouver le type.
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
  -- Débrief : lecture de debrief.log et envoi en morceaux
  ---------------------------------------------------------------------------
  -- Le fichier peut dépasser 1 Mo ; on l'envoie en morceaux base64 pour ne pas
  -- saturer la connexion ni bloquer le simulateur.
  local CHUNK_BYTES = 32768

  local function readFile(path)
    local f = io.open(path, "rb")
    if not f then return nil end
    local data = f:read("*a")
    f:close()
    return data
  end

  -- Encodage base64 (LuaSocket le fournit ; sinon on n'envoie pas).
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
      say("debrief.log introuvable ou vide")
      return
    end

    local chunks = math.ceil(#data / CHUNK_BYTES)
    local transferId = string.format("dbg-%d", os.time())
    local missionName = (Sim and Sim.getMissionName and Sim.getMissionName()) or "?"

    for i = 0, chunks - 1 do
      local part = data:sub(i * CHUNK_BYTES + 1, (i + 1) * CHUNK_BYTES)
      local encoded = b64(part)
      if not encoded then
        say("encodage base64 indisponible, débrief non envoyé")
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
    say(string.format("débrief envoyé (%d octets, %d morceaux)", #data, chunks))
  end

  ---------------------------------------------------------------------------
  -- Table de callbacks attendue par Sim.setUserCallbacks
  ---------------------------------------------------------------------------
  local dcsmm = {}

  function dcsmm.onSimulationStart()
    local name = (Sim and Sim.getMissionName and Sim.getMissionName()) or "?"
    refreshSlotTypes()
    sendLine(toJson({ type = "mission", phase = "start", name = name }))
    sendPlayers()
    say("mission démarrée : " .. tostring(name))
  end

  function dcsmm.onSimulationStop()
    -- On envoie le débrief AVANT de fermer la connexion : le backend attend
    -- le fichier pour l'historiser.
    pcall(sendDebrief)
    sendLine(toJson({ type = "mission", phase = "end" }))
    if conn then pcall(function() conn:close() end) end
    conn = nil
    say("mission terminée")
  end

  function dcsmm.onGameEvent(eventName, arg1, arg2, arg3, arg4)
    -- On transmet tous les arguments non nuls ; le backend les stocke tels quels.
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

    -- Un changement de slot ou une connexion modifie l'état des joueurs :
    -- on rafraîchit tout de suite pour que l'UI reste cohérente.
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

  -- Rafraîchissement périodique des statistiques, via le timer du simulateur.
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
  say("hooks chargés")
end
