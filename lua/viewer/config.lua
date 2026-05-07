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
  remote_endpoint = {
    host = "127.0.0.1",
    port = 7357,
  },
  probe_timeout_ms = 250,
  debounce_ms = 120,
  auto_start = false,
}

function M.merge(user_config)
  return vim.tbl_deep_extend("force", {}, M.defaults, user_config or {})
end

return M
