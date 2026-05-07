local protocol = require("viewer.protocol")

local M = {}
M.__index = M

local function new_transport()
  return setmetatable({
    tcp = nil,
    connected = false,
    pending = {},
    endpoint = nil,
    on_close = nil,
  }, M)
end

local function connect(endpoint, timeout_ms, callback)
  local tcp = vim.loop.new_tcp()
  local timer = vim.loop.new_timer()
  local finished = false

  local function finish(ok, err)
    if finished then
      return
    end

    finished = true
    timer:stop()
    timer:close()

    if not ok then
      tcp:close()
      callback(false, err)
      return
    end

    local transport = new_transport()
    transport.tcp = tcp
    transport.connected = true
    transport.endpoint = endpoint
    callback(true, transport)
  end

  tcp:connect(endpoint.host, endpoint.port, function(err)
    if err then
      finish(false, err)
      return
    end

    finish(true)
  end)

  timer:start(timeout_ms or 250, 0, vim.schedule_wrap(function()
    finish(false, "timeout")
  end))
end

local function write_to_tcp(tcp, data)
  if not tcp then
    return false, "tcp is nil"
  end

  local ok, err = pcall(function()
    tcp:write(data)
  end)
  if not ok then
    return false, err
  end

  return true
end

function M:send(message)
  if not self.connected or not self.tcp then
    return false, "not connected"
  end

  local encoded = protocol.encode(message)
  local ok, err = write_to_tcp(self.tcp, encoded)
  if not ok then
    return false, err
  end

  return true
end

function M:close()
  if self.tcp then
    self.tcp:close()
  end

  self.connected = false
  self.tcp = nil
end

return {
  new = new_transport,
  connect = connect,
}
