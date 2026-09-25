--[[
  DCS Mission Manager — Hooks/dcsmm.lua
  ------------------------------------------------------------------
  Placeholder de la Phase 2 : événements de jeu, joueurs et chat.

  À placer dans : Saved Games\DCS\Scripts\Hooks\dcsmm.lua

  Ce fichier sera enrichi avec les callbacks `onGameEvent`,
  `onPlayerConnect/Disconnect`, `onChatMessage`, ainsi que la collecte
  `net.get_stat` / `net.get_player_info` (dont l'UCID) pour les
  statistiques avancées.
]]

local dcsmm = {}

local function log(msg)
  if net and net.log then net.log("DCSMM: " .. tostring(msg)) end
end

function dcsmm.onSimulationStart()
  local mission = Sim and Sim.getMissionName and Sim.getMissionName() or "?"
  log("mission démarrée : " .. tostring(mission))
end

function dcsmm.onSimulationStop()
  log("mission terminée")
end

function dcsmm.onGameEvent(eventName, arg1, arg2, arg3, arg4)
  -- Phase 2 : transmettre les événements au backend.
  -- ex. kill, crash, eject, takeoff, landing, pilot_death, mission_end…
end

dcsmm.onPlayerConnect = function(id) log("joueur connecté : " .. tostring(id)) end
dcsmm.onPlayerDisconnect = function(id, code) log("joueur déconnecté : " .. tostring(id) .. " (" .. tostring(code) .. ")") end
dcsmm.onPlayerStart = function(id) log("joueur en simulation : " .. tostring(id)) end
dcsmm.onPlayerStop = function(id) log("joueur hors simulation : " .. tostring(id)) end
dcsmm.onPlayerChangeSlot = function(id) log("changement de slot : " .. tostring(id)) end

Sim.setUserCallbacks(dcsmm)
log("hooks chargés")
