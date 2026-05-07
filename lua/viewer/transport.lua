local protocol = require("viewer.protocol")

local M = {}
M.__index = M

local function new_transport()
  return setmetatable({
    tcp = nil,
    connected = false,
    closing = false,
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
    tcp:read_start(function(read_err, chunk)
      if transport.closing or not transport.connected then
        return
      end
      if read_err then
        transport:_notify_close(read_err)
        return
      end
      if chunk == nil then
        transport:_notify_close("eof")
      end
    end)
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

function M:_notify_close(reason)
  if not self.connected then
    return
  end

  self.connected = false
  self.tcp = nil
  if self.on_close then
    self.on_close(reason or "closed")
  end
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

function M:set_on_close(cb)
  self.on_close = cb
end

function M:close()
  self.closing = true
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
