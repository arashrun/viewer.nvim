if vim.g.loaded_viewer_nvim == 1 then
  return
end
vim.g.loaded_viewer_nvim = 1

local ok, viewer_or_err = pcall(require, "viewer")
if not ok then
  vim.schedule(function()
    vim.notify(
      "viewer.nvim load failed: " .. tostring(viewer_or_err) .. ". Make sure plugin/viewer.lua and lua/viewer/*.lua are both installed.",
      vim.log.levels.ERROR,
      { title = "viewer.nvim" }
    )
  end)
  return
end

local viewer = viewer_or_err

viewer.setup({})

vim.api.nvim_create_user_command("ViewerPreview", function()
  viewer.preview()
end, {})

vim.api.nvim_create_user_command("ViewerToggle", function()
  viewer.toggle()
end, {})

vim.api.nvim_create_user_command("ViewerInterval", function(opts)
  viewer.set_interval(opts.args)
end, {
  nargs = 1,
  complete = "customlist",
})
