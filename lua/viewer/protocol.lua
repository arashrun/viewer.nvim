local M = {}

local function json_encode(value)
  if vim.json and vim.json.encode then
    return vim.json.encode(value)
  end

  return vim.fn.json_encode(value)
end

local function json_decode(value)
  if vim.json and vim.json.decode then
    return vim.json.decode(value)
  end

  return vim.fn.json_decode(value)
end

function M.encode(message)
  return json_encode(message) .. "\n"
end

function M.decode(line)
  if not line or line == "" then
    return nil
  end

  local ok, decoded = pcall(json_decode, line)
  if not ok then
    return nil, decoded
  end

  return decoded
end

function M.hello(payload)
  return {
    type = "hello",
    payload = payload or {},
  }
end

function M.preview(payload)
  return {
    type = "preview",
    payload = payload or {},
  }
end

function M.session(payload)
  return {
    type = "session",
    payload = payload or {},
  }
end

function M.focus(state)
  return {
    type = "focus",
    payload = { focused = not not state },
  }
end

function M.viewport(payload)
  return {
    type = "viewport",
    payload = payload or {},
  }
end

function M.close()
  return {
    type = "close",
  }
end

return M
