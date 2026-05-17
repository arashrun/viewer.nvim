local config = require("viewer.config")
local protocol = require("viewer.protocol")
local transport_mod = require("viewer.transport")

local M = {}

local state = {
  config = config.merge(),
  transport = nil,
  active = false,
  session_kind = nil,
  bufnr = nil,
  session_id = nil,
  last_docs_query = nil,
  preview_timer = nil,
  viewport_timer = nil,
  reconnect_timer = nil,
  reconnecting = false,
  spawning = false,
  mapped_keys = {},
}

local reconnect_delay_ms = 1000

local function is_markdown_buffer(bufnr)
  local ft = vim.bo[bufnr].filetype
  return state.config.enabled_filetypes[ft] == true
end

local function new_session_id()
  return string.format("%d:%d:%d", vim.loop.hrtime(), vim.fn.getpid(), math.random(100000, 999999))
end

local function ensure_session_id()
  if not state.session_id then
    state.session_id = new_session_id()
  end
  return state.session_id
end

local function normalize_interval_ms(value)
  local ms = tonumber(value)
  if not ms or ms <= 0 then
    return nil
  end
  return math.floor(ms)
end

local function buffer_base_dir(bufnr)
  local path = vim.api.nvim_buf_get_name(bufnr)
  if not path or path == "" then
    return nil
  end
  return vim.fs.dirname(path)
end

local function image_mime_from_path(path)
  local ext = vim.fn.fnamemodify(path, ":e"):lower()
  if ext == "jpg" or ext == "jpeg" then
    return "image/jpeg"
  end
  if ext == "png" then
    return "image/png"
  end
  if ext == "gif" then
    return "image/gif"
  end
  if ext == "webp" then
    return "image/webp"
  end
  if ext == "svg" then
    return "image/svg+xml"
  end
  if ext == "avif" then
    return "image/avif"
  end
  if ext == "bmp" then
    return "image/bmp"
  end
  if ext == "tif" or ext == "tiff" then
    return "image/tiff"
  end
  return "application/octet-stream"
end

local function encode_base64_blob(blob)
  if vim.base64 and vim.base64.encode then
    return vim.base64.encode(blob)
  end
  return nil
end

local function read_binary_file(path)
  local fd = vim.loop.fs_open(path, "r", 438)
  if not fd then
    return nil
  end
  local stat = vim.loop.fs_fstat(fd)
  if not stat or not stat.size or stat.size <= 0 then
    vim.loop.fs_close(fd)
    return nil
  end

  local data = vim.loop.fs_read(fd, stat.size, 0)
  vim.loop.fs_close(fd)
  return data
end

local function resolve_image_target(base_dir, raw_target)
  local target = raw_target:gsub("^%s+", ""):gsub("%s+$", "")
  if target:sub(1, 1) == "<" and target:sub(-1) == ">" then
    target = target:sub(2, -2)
  else
    local dest = target:match("^(.-)%s+[\"']")
    if dest and dest ~= "" then
      target = dest
    end
  end
  target = target:gsub('\\"', '"')
  if target == "" then
    return nil
  end
  if target:match("^https?://") or target:match("^data:") then
    return nil
  end
  if target:match("^/") or target:match("^%a:[/\\]") then
    return target
  end
  if not base_dir or base_dir == "" then
    return target
  end
  return vim.fs.joinpath(base_dir, target)
end

local function collect_markdown_resources(bufnr, lines)
  local base_dir = buffer_base_dir(bufnr)
  if not base_dir then
    return {}
  end

  local seen = {}
  local resources = {}

  for _, line in ipairs(lines) do
    for raw in line:gmatch("!%b[]%(([^)]-)%)") do
      local dest = raw:gsub("^%s+", ""):gsub("%s+$", "")
      if dest:sub(1, 1) == "<" and dest:sub(-1) == ">" then
        dest = dest:sub(2, -2)
      else
        local stripped = dest:match("^(.-)%s+[\"']")
        if stripped and stripped ~= "" then
          dest = stripped
        end
      end

      local target = resolve_image_target(base_dir, dest)
      if target and not seen[target] then
        seen[target] = true
        local binary = read_binary_file(target)
        if binary then
          local encoded = encode_base64_blob(binary)
          if encoded then
            resources[#resources + 1] = {
              src = dest,
              path = target,
              mime = image_mime_from_path(target),
              data = encoded,
            }
          end
        end
      end
    end
  end

  return resources
end

local function notify(msg, level)
  vim.schedule(function()
    vim.notify(msg, level or vim.log.levels.INFO, { title = "viewer.nvim" })
  end)
end

local function current_buffer_filetype()
  return vim.bo[vim.api.nvim_get_current_buf()].filetype
end

local function clear_mapped_key(key)
  local mapped = state.mapped_keys[key]
  if not mapped then
    return
  end
  pcall(vim.keymap.del, "n", mapped)
  state.mapped_keys[key] = nil
end

local function set_mapped_key(key, lhs, rhs, desc)
  clear_mapped_key(key)
  if lhs == false then
    return
  end
  local effective_lhs = lhs
  if type(effective_lhs) ~= "string" or effective_lhs == "" then
    return
  end
  vim.keymap.set("n", effective_lhs, rhs, {
    desc = desc,
    silent = true,
  })
  state.mapped_keys[key] = effective_lhs
end

local function setup_keymaps()
  local keymaps = state.config.keymaps or {}
  set_mapped_key("preview", keymaps.preview or "<leader>vp", function()
    M.preview()
  end, "Start markdown preview")
  set_mapped_key("toggle", keymaps.toggle or "<leader>vt", function()
    M.toggle()
  end, "Toggle preview")
  set_mapped_key("interval", keymaps.interval or "<leader>vi", function()
    vim.ui.input({ prompt = "Auto hide interval (ms): " }, function(value)
      if value and value ~= "" then
        M.set_interval(value)
      end
    end)
  end, "Set auto hide interval")
  set_mapped_key("docs", keymaps.docs or "<leader>vd", function()
    M.docs_query_current_word()
  end, "Lookup offline docs for current word")
end

local function endpoint_equals(a, b)
  return a
    and b
    and a.host == b.host
    and a.port == b.port
end

local function has_custom_remote_endpoint()
  return not endpoint_equals(state.config.remote_endpoint, config.defaults.remote_endpoint)
end

local function endpoint_order()
  local remote = state.config.remote_endpoint
  local local_ep = state.config.local_endpoint
  local is_ssh = vim.env.SSH_CONNECTION or vim.env.SSH_CLIENT or vim.env.SSH_TTY
  local custom_remote = has_custom_remote_endpoint()

  if custom_remote or is_ssh then
    return {
      { endpoint = remote, spawnable = false },
      { endpoint = local_ep, spawnable = false },
    }
  end

  return {
    { endpoint = local_ep, spawnable = true },
    { endpoint = remote, spawnable = false },
  }
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

local function with_nview_args(cmd)
  local result = {}
  for i = 1, #cmd do
    result[#result + 1] = cmd[i]
  end

  local auto_hide_arg = string.format("-auto-hide-ms=%d", tonumber(state.config.auto_hide_ms) or 3000)
  local has_auto_hide = false
  for i = 1, #result do
    if tostring(result[i]):match("^%-auto%-hide%-ms=") then
      has_auto_hide = true
      break
    end
  end
  if not has_auto_hide then
    result[#result + 1] = auto_hide_arg
  end

  return result
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

  cmd = with_nview_args(cmd)

  state.spawning = true
  vim.schedule(function()
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
  end)
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
  stop_preview_timer()
  stop_viewport_timer()
  stop_reconnect_timer()
  if state.transport then
    state.transport:set_on_close(nil)
    state.transport:close()
  end
  state.transport = nil
  state.active = false
  state.session_kind = nil
  state.reconnecting = false
end

local schedule_preview_sync
local schedule_viewport_sync
local attach_autocmds

local function install_reconnect_handler(retry_fn)
  if not state.transport then
    return
  end
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
      state.reconnect_timer:start(reconnect_delay_ms, reconnect_delay_ms, vim.schedule_wrap(function()
        if state.active or not state.reconnecting then
          return
        end
        retry_fn()
      end))
    end
  end)
end

local function send_common_session_messages()
  if not state.transport then
    return
  end
  state.transport:send(protocol.hello({
    plugin = "viewer.nvim",
    version = "0.1.0",
    session_id = ensure_session_id(),
  }))
  state.transport:send(protocol.session({
    session_id = ensure_session_id(),
  }))
  state.transport:send(protocol.interval(state.config.auto_hide_ms))
end

local function connect_session(bufnr, transport)
  state.transport = transport
  state.active = true
  state.session_kind = "markdown"
  state.reconnecting = false
  ensure_session_id()
  install_reconnect_handler(function()
    M.preview()
  end)
  send_common_session_messages()
  state.transport:send(protocol.preview({
    bufnr = bufnr,
    path = vim.api.nvim_buf_get_name(bufnr),
    filetype = vim.bo[bufnr].filetype,
    lines = vim.api.nvim_buf_get_lines(bufnr, 0, -1, false),
    session_id = ensure_session_id(),
  }))
  attach_autocmds()
  schedule_preview_sync()
  schedule_viewport_sync()
  notify("preview started")
end

local function connect_docs_session(query, transport)
  state.transport = transport
  state.active = true
  state.session_kind = "docs"
  state.reconnecting = false
  state.last_docs_query = query
  ensure_session_id()
  install_reconnect_handler(function()
    M.docs_query(query)
  end)
  send_common_session_messages()
  state.transport:send(protocol.docs_query({
    query = query,
    filetype = current_buffer_filetype(),
    session_id = ensure_session_id(),
  }))
  notify("docs query started")
end

schedule_preview_sync = function()
  if not state.active or not state.transport or state.session_kind ~= "markdown" then
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
    local resources = collect_markdown_resources(bufnr, lines)
    local payload = {
      bufnr = bufnr,
      path = vim.api.nvim_buf_get_name(bufnr),
      filetype = vim.bo[bufnr].filetype,
      lines = lines,
      line_count = #lines,
      cursor = { row = cursor[1], col = cursor[2] },
      viewport = { width = width, height = height },
      resources = resources,
      session_id = ensure_session_id(),
    }

    state.transport:send(protocol.preview(payload))
  end))
end

schedule_viewport_sync = function()
  if not state.active or not state.transport or state.session_kind ~= "markdown" then
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
    local lines = vim.api.nvim_buf_get_lines(bufnr, 0, -1, false)
    local payload = {
      bufnr = bufnr,
      path = vim.api.nvim_buf_get_name(bufnr),
      filetype = vim.bo[bufnr].filetype,
      resources = collect_markdown_resources(bufnr, lines),
      cursor = { row = cursor[1], col = cursor[2] },
      viewport = { width = width, height = height },
      session_id = ensure_session_id(),
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
        if state.session_kind ~= "markdown" or not is_markdown_buffer(args.buf) then
          return
        end
        schedule_preview_sync()
      end

      if (args.event == "TextChanged" or args.event == "TextChangedI") and state.session_kind == "markdown" then
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
  local endpoints = endpoint_order()

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
  math.randomseed(vim.loop.hrtime() % 2147483647)
  setup_keymaps()

  if state.config.auto_start then
    vim.schedule(function()
      local bufnr = vim.api.nvim_get_current_buf()
      if is_markdown_buffer(bufnr) then
        start_preview(bufnr)
      end
    end)
  end
end

function M.set_interval(ms)
  local normalized = normalize_interval_ms(ms)
  if not normalized then
    notify("ViewerInterval expects a positive integer", vim.log.levels.ERROR)
    return
  end

  if normalized < 1000 then
    normalized = normalized * 1000
  end

  state.config.auto_hide_ms = normalized
  if state.transport then
    state.transport:send(protocol.interval(normalized))
  end
  notify("auto hide interval set to " .. normalized .. " ms")
end

function M.preview()
  if state.active and state.transport then
    clear_transport()
  end
  start_preview(vim.api.nvim_get_current_buf())
end

function M.docs_query(query)
  local normalized = vim.trim(query or "")
  if normalized == "" then
    normalized = vim.trim(vim.fn.expand("<cword>"))
  end
  if normalized == "" then
    notify("ViewerDocsQuery expects a non-empty query", vim.log.levels.ERROR)
    return
  end

  state.last_docs_query = normalized
  local filetype = current_buffer_filetype()
  if state.active and state.transport then
    state.session_kind = "docs"
    stop_preview_timer()
    stop_viewport_timer()
    install_reconnect_handler(function()
      M.docs_query(normalized)
    end)
    state.transport:send(protocol.docs_query({
      query = normalized,
      filetype = filetype,
      session_id = ensure_session_id(),
    }))
    notify("docs query: " .. normalized)
    return
  end

  pick_endpoint(function(ok, transport_or_err)
    if not ok then
      notify("failed to connect to nview: " .. transport_or_err, vim.log.levels.ERROR)
      return
    end

    stop_reconnect_timer()
    connect_docs_session(normalized, transport_or_err)
  end)
end

function M.docs_query_current_word()
  local query = vim.trim(vim.fn.expand("<cword>"))
  if query == "" then
    notify("current word is empty", vim.log.levels.ERROR)
    return
  end
  M.docs_query(query)
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
