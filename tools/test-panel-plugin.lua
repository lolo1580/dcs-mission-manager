-- Run from the workspace root with Lua 5.1 or Fengari. Device/socket fixtures
-- verify lifecycle behaviour; they cannot prove an action works inside DCS.
local now, name = 0, "FA-18C_hornet"
local packets, replies, calls = {}, {}, {}
local closed = false
local listener = {
  settimeout = function(_, value) assert(value == 0) end,
  setsockname = function(_, ip, port) assert(ip == "127.0.0.1" and port == 7780); return true end,
  receivefrom = function() local p = table.remove(packets,1); if p then return p[1],p[2],p[3] end end,
  sendto = function(_, text) replies[#replies+1] = text end,
  close = function() closed = true end,
}
LoGetSelfData = function() return {Name = name} end
GetDevice = function(id)
  assert(id==13)
  return {performClickableAction=function(_,command,value) calls[#calls+1]={command,value} end}
end
local plugin = dofile("dcs-lua/PanelCommands.lua")({udp=function() return listener end,gettime=function() return now end},function()end)
local token = string.rep("a",32)
local function packet(seq, command, ip) packets[#packets+1]={"DCSM1 "..token.." "..seq.." "..command,ip or "127.0.0.1",12345} end
local function last(pattern) assert(replies[#replies]:find(pattern), replies[#replies]) end
plugin.start()
packet(1,"TRIM UP");plugin.frame();last("ERR_SESSION");assert(#calls==0)
packet(2,"PING");plugin.frame();last("PONG FA%-18C_hornet")
packet(3,"TRIM UP");plugin.frame();last("OK");assert(calls[1][1]==3014 and calls[1][2]==1)
now=0.049;plugin.frame();assert(#calls==1)
now=0.051;plugin.frame();assert(calls[2][1]==3014 and calls[2][2]==0)
packet(3,"TRIM UP");plugin.frame();last("ERR_SEQUENCE");assert(#calls==2)
packet(4,"TRIM DN","192.168.0.1");plugin.frame();assert(#calls==2)
packet(4,"os.execute('bad')");plugin.frame();last("ERR_COMMAND")
packet(5,"TRIM DN");plugin.frame();assert(calls[3][1]==3015 and calls[3][2]==1)
packet(6,"TRIM UP");plugin.frame()
name="F-16C_50";plugin.frame();last("ERR_AIRCRAFT");assert(calls[4][2]==0)
packet(7,"TRIM UP");plugin.frame();last("ERR_AIRCRAFT");assert(#calls==4)
name="FA-18C_hornet";packet(8,"TRIM UP");plugin.frame();assert(#calls==5)
plugin.stop();assert(closed and calls[6][2]==0)
plugin.start();packet(9,"PING");plugin.frame()
for seq=10,20 do packet(seq,"TRIM UP") end
plugin.frame();assert(#calls==7)
now=2;plugin.frame();assert(calls[8][2]==0);last("ERR_EXPIRED")
now=4;plugin.frame();last("ERR_TIMEOUT")
packet(21,"TRIM UP");plugin.frame();last("ERR_SESSION")
packet(22,"PING");plugin.frame()
packet(23,"TRIM UP");plugin.frame();local beforeCancel=#calls
packet(24,"TRIM DN");plugin.frame()
packet(25,"CANCEL");plugin.frame();last("CANCELLED");assert(#calls==beforeCancel+1 and calls[#calls][2]==0)
now=4.2;plugin.frame();assert(#calls==beforeCancel+1)
local failRelease=true
GetDevice=function()
  return {performClickableAction=function(_,command,value)
    if value==0 and failRelease then error("simulated release failure") end
    calls[#calls+1]={command,value}
  end}
end
packet(26,"TRIM UP");plugin.frame();local beforeFailure=#calls
now=4.3;plugin.frame();last("ERR_RELEASE");assert(#calls==beforeFailure)
packet(27,"TRIM DN");plugin.frame();last("ERR_RELEASE");assert(#calls==beforeFailure)
failRelease=false;now=4.4;plugin.frame();assert(calls[#calls][2]==0)
plugin.stop()
print("OK panel plugin: directions, release, aircraft, session, duplicates, queue, expiry, stop, cancel, release failure retry")
