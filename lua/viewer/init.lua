local config = require("viewer.config")
local protocol = require("viewer.protocol")
local transport_mod = require("viewer.transport")

local M = {}

local state = {
  config = config.merge(),
  transport = nil,
  active = false,
  bufnr = nil,
  timer = nil,
  reconnect_timer = nil,
  last_focused = true,
  reconnecting = false,
}

local function is_markdown_buffer(bufnr)
  local ft = vim.bo[bufnr].filetype
  return state.config.enabled_filetypes[ft] == true
end

local function notify(msg, level)
  vim.schedule(function()
    vim.notify(msg, level or vim.log.levels.INFO, { title = "viewer.nvim" })
  end)
end

local function current_endpoint()
  local remote = state.config.remote_endpoint
  local local_ep = state.config.local_endpoint

  if vim.env.SSH_CONNECTION or vim.env.SSH_CLIENT or vim.env.SSH_TTY then
    return remote, local_ep
  end

  return local_ep, remote
end

local function stop_timer()
  if state.timer then
    state.timer:stop()
    state.timer:close()
    state.timer = nil
  end
end

local function stop_reconnect_timer()
  if state.reconnect_timer then
    state.reconnect_timer:stop()
    state.reconnect_timer:close()
    state.reconnect_timer = nil
  end
end

local function clear_transport()
  if state.transport then
    state.transport:set_on_close(nil)
    state.transport:close()
  end
  state.transport = nil
  state.active = false
  state.reconnecting = false
end

local schedule_sync
local attach_autocmds
local send_focus

local function connect_session(bufnr, transport)
  state.transport = transport
  state.active = true
  state.reconnecting = false
  state.transport:set_on_close(function()
    if not state.active then
      return
    end
    state.transport = nil
    state.active = false
    if not state.reconnecting then
      state.reconnecting = true
      notify("nview disconnected, retrying...", vim.log.levels.WARN)
      stop_reconnect_timer()
      state.reconnect_timer = vim.loop.new_timer()
      state.reconnect_timer:start(1000, 1000, vim.schedule_wrap(function()
        if state.active or not state.reconnecting then
          return
        end
        M.preview()
      end))
    end
  end)

  state.transport:send(protocol.hello({
    plugin = "viewer.nvim",
    version = "0.1.0",
  }))
  state.transport:send(protocol.preview({
    bufnr = bufnr,
    path = vim.api.nvim_buf_get_name(bufnr),
    filetype = vim.bo[bufnr].filetype,
    lines = vim.api.nvim_buf_get_lines(bufnr, 0, -1, false),
  }))
  attach_autocmds()
  send_focus(true)
  schedule_sync()
  notify("preview started")
end

schedule_sync = function()
  if not state.active or not state.transport then
    return
  end

  stop_timer()
  state.timer = vim.loop.new_timer()
  state.timer:start(state.config.debounce_ms, 0, vim.schedule_wrap(function()
    if not state.active or not state.transport then
      return
    end

    local bufnr = state.bufnr
    if not bufnr or not vim.api.nvim_buf_is_valid(bufnr) then
      return
    end

    local lines = vim.api.nvim_buf_get_lines(bufnr, 0, -1, false)
    local winid = vim.api.nvim_get_current_win()
    local cursor = vim.api.nvim_win_get_cursor(winid)
    local width = vim.api.nvim_win_get_width(winid)
    local height = vim.api.nvim_win_get_height(winid)
    local payload = {
      bufnr = bufnr,
      path = vim.api.nvim_buf_get_name(bufnr),
      filetype = vim.bo[bufnr].filetype,
      lines = lines,
      cursor = { row = cursor[1], col = cursor[2] },
      viewport = { width = width, height = height },
    }

    state.transport:send(protocol.preview(payload))
    state.transport:send(protocol.viewport(payload))
  end))
end

send_focus = function(is_focused)
  if state.transport then
    state.transport:send(protocol.focus(is_focused))
  end
  state.last_focused = is_focused
end

attach_autocmds = function()
  local group = vim.api.nvim_create_augroup("ViewerNvim", { clear = true })

  vim.api.nvim_create_autocmd({ "TextChanged", "TextChangedI", "BufEnter", "CursorMoved", "WinResized", "VimResized" }, {
    group = group,
    callback = function(args)
      if not state.active then
        return
      end

      if args.event == "BufEnter" then
        state.bufnr = args.buf
        if not is_markdown_buffer(args.buf) then
          return
        end
      end

      schedule_sync()
    end,
  })

  vim.api.nvim_create_autocmd({ "FocusGained", "FocusLost" }, {
    group = group,
    callback = function(args)
      if not state.active then
        return
      end

      send_focus(args.event == "FocusGained")
      if args.event == "FocusGained" then
        schedule_sync()
      end
    end,
  })
end

local function pick_endpoint(callback)
  local first, second = current_endpoint()
  local endpoints = { first, second }

  local function try_next(index)
    local endpoint = endpoints[index]
    if not endpoint then
      callback(false, "nview not found")
      return
    end

    transport_mod.connect(endpoint, state.config.probe_timeout_ms, function(ok, transport_or_err)
      if ok then
        vim.schedule(function()
          callback(true, transport_or_err)
        end)
        return
      end

      vim.schedule(function()
        try_next(index + 1)
      end)
    end)
  end

  try_next(1)
end

local function start_preview(bufnr)
  if not is_markdown_buffer(bufnr) then
    notify("current buffer is not a supported markdown filetype", vim.log.levels.WARN)
    return
  end

  state.bufnr = bufnr
  pick_endpoint(function(ok, transport_or_err)
    if not ok then
      notify("failed to connect to nview: " .. transport_or_err, vim.log.levels.ERROR)
      return
    end

    stop_reconnect_timer()
    connect_session(bufnr, transport_or_err)
  end)
end

function M.setup(user_config)
  state.config = config.merge(user_config)

  if state.config.auto_start then
    vim.schedule(function()
      local bufnr = vim.api.nvim_get_current_buf()
      if is_markdown_buffer(bufnr) then
        start_preview(bufnr)
      end
    end)
  end
end

function M.preview()
  start_preview(vim.api.nvim_get_current_buf())
end

function M.toggle()
  if state.active then
    stop_reconnect_timer()
    stop_timer()
    if state.transport then
      state.transport:set_on_close(nil)
      state.transport:send(protocol.close())
    end
    clear_transport()
    notify("preview stopped")
    return
  end

  M.preview()
end

return M
