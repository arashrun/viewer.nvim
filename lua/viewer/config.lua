local M = {}

M.defaults = {
  enabled_filetypes = {
    markdown = true,
    md = true,
    rmd = true,
    quarto = true,
  },
  local_endpoint = {
    host = "127.0.0.1",
    port = 7357,
  },
  remote_endpoints = {},
  remote_endpoint = {
    host = "127.0.0.1",
    port = 7357,
  },
  probe_timeout_ms = 1000,
  debounce_ms = 120,
  auto_hide_ms = 3000,
  auto_start = false,
  nview_cmd = nil,
  keymaps = {
    preview = "<leader>vp",
    toggle = "<leader>vt",
    interval = "<leader>vi",
    docs = "<leader>vd",
  },
}

function M.merge(user_config)
  return vim.tbl_deep_extend("force", {}, M.defaults, user_config or {})
end

return M
