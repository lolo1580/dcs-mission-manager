-- Loaded by the managed Export.lua block. No arbitrary Lua or device commands
-- are accepted over the socket. DCS 2.9.30 Hornet HOTAS IDs are aircraft-specific.
return function(socket, say)
  local listener, owner, ownerAt, lastSeq
  local queue, active = {}, nil
  local function aircraft()
    local ok, data = pcall(LoGetSelfData)
    return ok and data and data.Name or "NONE"
  end
  local function reply(item, status)
    if listener then
      listener:sendto("DCSM1 " .. item.token .. " " .. item.seq .. " " .. status .. " " .. aircraft(), item.ip, item.port)
    end
  end
  local function release()
    if active then
      -- Use the device which was pressed, even after ownship changes.
      local ok, err = pcall(function() active.device:performClickableAction(active.command, 0) end)
      if not ok then
        if not active.releaseFailed then say("panel trim release failed: " .. tostring(err)); reply(active.item, "ERR_RELEASE") end
        active.releaseFailed = true
        active.untilAt = socket.gettime() + 0.05
        return false
      end
      active = nil
    end
    return true
  end
  local function clear(reason)
    release()
    for _, item in ipairs(queue) do reply(item, "ERR_" .. reason) end
    queue = {}
  end
  return {
    start = function()
      listener = socket.udp()
      if not listener then say("panel command socket unavailable"); return end
      listener:settimeout(0)
      local ok, err = listener:setsockname("127.0.0.1", 7780)
      if not ok then listener:close(); listener = nil; say("panel command listener failed: " .. tostring(err)); return end
      say("panel command plugin listening on 127.0.0.1:7780")
    end,
    stop = function()
      clear("STOPPED")
      if listener then listener:close(); listener = nil end
      owner, ownerAt, lastSeq = nil, nil, nil
    end,
    frame = function()
      if not listener then return end
      local now = socket.gettime()
      if aircraft() ~= "FA-18C_hornet" then clear("AIRCRAFT") end
      if ownerAt and now - ownerAt > 3 then clear("TIMEOUT"); owner = nil end
      if active and now >= active.untilAt then release() end
      -- Bounded, non-blocking work. A detent never sleeps inside a DCS frame.
      for _ = 1, 16 do
        local packet, ip, port = listener:receivefrom(256)
        if not packet then break end
        if ip == "127.0.0.1" then
          local token, seq, command = packet:match("^DCSM1 ([0-9a-f]+) (%d+) (.+)$")
          seq = tonumber(seq)
          if token and #token == 32 and seq and seq > 0 and seq <= 9007199254740991 then
            local item = {token = token, seq = seq, ip = ip, port = port}
            local key = token .. ":" .. port
            if command == "PING" then
              if owner ~= key then clear("SESSION"); lastSeq = 0 end
              if seq > lastSeq then
                owner, ownerAt, lastSeq = key, now, seq
                reply(item, "PONG")
              end
            elseif command == "CANCEL" then
              if owner ~= key then reply(item, "ERR_SESSION")
              elseif seq <= lastSeq then reply(item, "ERR_SEQUENCE")
              else
                lastSeq = seq
                clear("CANCELLED")
                reply(item, active and "ERR_RELEASE" or "CANCELLED")
              end
            elseif command == "TRIM UP" or command == "TRIM DN" then
              if owner ~= key then reply(item, "ERR_SESSION")
              elseif seq <= lastSeq then reply(item, "ERR_SEQUENCE")
              else
                lastSeq = seq
                if aircraft() ~= "FA-18C_hornet" then reply(item, "ERR_AIRCRAFT")
                elseif active and active.releaseFailed then reply(item, "ERR_RELEASE")
                elseif #queue >= 8 then reply(item, "ERR_BUSY")
                else
                  item.command = command == "TRIM UP" and 3014 or 3015
                  item.expires = now + 1
                  queue[#queue + 1] = item
                end
              end
            else reply(item, "ERR_COMMAND") end
          end
        end
      end
      if not active and #queue > 0 then
        local item = table.remove(queue, 1)
        if now > item.expires then reply(item, "ERR_EXPIRED")
        else
          local device
          local ok, err = pcall(function()
            device = GetDevice(13)
            device:performClickableAction(item.command, 1)
          end)
          if ok then
            active = {device = device, command = item.command, untilAt = now + 0.05, item = item}
            reply(item, "OK")
          else
            -- A failing call may have partially pressed the device.
            if device then pcall(function() device:performClickableAction(item.command, 0) end) end
            reply(item, "ERR_DEVICE")
            say("panel trim press failed: " .. tostring(err))
          end
        end
      end
    end,
  }
end
