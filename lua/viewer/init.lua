local config = require("viewer.config")
local protocol = require("viewer.protocol")
local transport_mod = require("viewer.transport")

local M = {}

local state = {
  config = config.merge(),
  transport = nil,
  active = false,
  bufnr = nil,
  preview_timer = nil,
  viewport_timer = nil,
  reconnect_timer = nil,
  reconnecting = false,
  spawning = false,
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

local function resolve_nview_command()
  local user_cmd = state.config.nview_cmd
  if type(user_cmd) == "table" and #user_cmd > 0 then
    return user_cmd
  end

  local source = debug.getinfo(1, "S").source
  if type(source) == "string" and source:sub(1, 1) == "@" then
    local init_path = source:sub(2)
    local root = vim.fs.dirname(vim.fs.dirname(vim.fs.dirname(init_path)))
    local exe_name = vim.fn.has("win32") == 1 and "nview.exe" or "nview"
    local candidate = vim.fs.joinpath(root, "bin", exe_name)
    if vim.fn.executable(candidate) == 1 then
      return { candidate }
    end
  end

  if vim.fn.executable("nview") == 1 then
    return { "nview" }
  end

  return nil
end

local function spawn_nview(callback)
  if state.spawning then
    callback(false, "nview is starting")
    return
  end

  local cmd = resolve_nview_command()
  if not cmd then
    callback(false, "nview executable not found")
    return
  end

  state.spawning = true
  local job_id = vim.fn.jobstart(cmd, { detach = true })
  if job_id <= 0 then
    state.spawning = false
    callback(false, "failed to start nview")
    return
  end

  vim.defer_fn(function()
    state.spawning = false
    callback(true)
  end, 500)
end

local function stop_preview_timer()
  if state.preview_timer then
    state.preview_timer:stop()
    state.preview_timer:close()
    state.preview_timer = nil
  end
end

local function stop_viewport_timer()
  if state.viewport_timer then
    state.viewport_timer:stop()
    state.viewport_timer:close()
    state.viewport_timer = nil
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

local schedule_preview_sync
local schedule_viewport_sync
local attach_autocmds

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
  schedule_preview_sync()
  schedule_viewport_sync()
  notify("preview started")
end

schedule_preview_sync = function()
  if not state.active or not state.transport then
    return
  end

  stop_preview_timer()
  state.preview_timer = vim.loop.new_timer()
  state.preview_timer:start(state.config.debounce_ms, 0, vim.schedule_wrap(function()
    if not state.active or not state.transport then
      return
    end

    local bufnr = state.bufnr
    if not bufnr or not vim.api.nvim_buf_is_valid(bufnr) then
      return
    end

    local lines = vim.api.nvim_buf_get_lines(bufnr, 0, -1, false)
    local cursor = vim.api.nvim_win_get_cursor(vim.api.nvim_get_current_win())
    local width = vim.api.nvim_win_get_width(vim.api.nvim_get_current_win())
    local height = vim.api.nvim_win_get_height(vim.api.nvim_get_current_win())
    local payload = {
      bufnr = bufnr,
      path = vim.api.nvim_buf_get_name(bufnr),
      filetype = vim.bo[bufnr].filetype,
      lines = lines,
      line_count = #lines,
      cursor = { row = cursor[1], col = cursor[2] },
      viewport = { width = width, height = height },
    }

    state.transport:send(protocol.preview(payload))
  end))
end

schedule_viewport_sync = function()
  if not state.active or not state.transport then
    return
  end

  stop_viewport_timer()
  state.viewport_timer = vim.loop.new_timer()
  state.viewport_timer:start(state.config.debounce_ms, 0, vim.schedule_wrap(function()
    if not state.active or not state.transport then
      return
    end

    local bufnr = state.bufnr
    if not bufnr or not vim.api.nvim_buf_is_valid(bufnr) then
      return
    end

    local winid = vim.api.nvim_get_current_win()
    local cursor = vim.api.nvim_win_get_cursor(winid)
    local width = vim.api.nvim_win_get_width(winid)
    local height = vim.api.nvim_win_get_height(winid)
    local payload = {
      bufnr = bufnr,
      path = vim.api.nvim_buf_get_name(bufnr),
      filetype = vim.bo[bufnr].filetype,
      cursor = { row = cursor[1], col = cursor[2] },
      viewport = { width = width, height = height },
    }

    state.transport:send(protocol.viewport(payload))
  end))
end

attach_autocmds = function()
  local group = vim.api.nvim_create_augroup("ViewerNvim", { clear = true })

  vim.api.nvim_create_autocmd({
    "TextChanged",
    "TextChangedI",
    "BufEnter",
    "CursorMoved",
    "CursorMovedI",
    "WinResized",
    "VimResized",
  }, {
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
        schedule_preview_sync()
      end

      if args.event == "TextChanged" or args.event == "TextChangedI" then
        schedule_preview_sync()
      end

      if args.event == "CursorMoved"
        or args.event == "CursorMovedI"
        or args.event == "WinResized"
        or args.event == "VimResized" then
        schedule_viewport_sync()
      end
    end,
  })

end

local function pick_endpoint(callback)
  local first, second = current_endpoint()
  local endpoints = {
    { endpoint = first, spawnable = vim.env.SSH_CONNECTION == nil and vim.env.SSH_CLIENT == nil and vim.env.SSH_TTY == nil },
    { endpoint = second, spawnable = false },
  }

  local function try_next(index, spawned)
    local item = endpoints[index]
    if not item then
      callback(false, "nview not found")
      return
    end

    local endpoint = item.endpoint
    if not endpoint then
      try_next(index + 1, false)
      return
    end

    transport_mod.connect(endpoint, state.config.probe_timeout_ms, function(ok, transport_or_err)
      if ok then
        vim.schedule(function()
          callback(true, transport_or_err)
        end)
        return
      end

      if item.spawnable and not spawned then
        spawn_nview(function(spawn_ok, spawn_err)
          if not spawn_ok then
            vim.schedule(function()
              try_next(index + 1, false)
            end)
            return
          end

          vim.defer_fn(function()
            try_next(index, true)
          end, 1000)
        end)
        return
      end

      vim.schedule(function()
        try_next(index + 1, false)
      end)
    end)
  end

  try_next(1, false)
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
    stop_preview_timer()
    stop_viewport_timer()
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
