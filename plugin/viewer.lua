if vim.g.loaded_viewer_nvim == 1 then
  return
end
vim.g.loaded_viewer_nvim = 1

local viewer = require("viewer")

viewer.setup({})

vim.api.nvim_create_user_command("ViewerPreview", function()
  viewer.preview()
end, {})

vim.api.nvim_create_user_command("ViewerToggle", function()
  viewer.toggle()
end, {})
