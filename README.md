## 编译与安装

### 依赖

- Go 1.22+(当前开发环境,go相关的编译工具在 `~/bin/go/bin/` 目录)
- Neovim 0.10+ 

### 编译 `nview`

```bash
go build -o ./bin/nview ./cmd/nview
```

如果想直接安装到 Go 的 `bin` 目录:

```bash
go install ./cmd/nview
```

### Windows 编译

在 Windows 上本机编译:

```powershell
go build -o .\nview.exe .\cmd\nview
```

交叉编译 Windows 64 位版本:

```bash
GOOS=windows GOARCH=amd64 go build -o nview.exe ./cmd/nview
```

交叉编译 Windows ARM64 版本:

```bash
GOOS=windows GOARCH=arm64 go build -o nview.exe ./cmd/nview
```

### 安装 `viewer.nvim`

把仓库加入 Neovim 的 runtimepath 即可, 例如使用手动拷贝或插件管理器加载整个仓库。

如果手动安装, 目录结构至少要保留:

- `plugin/viewer.lua`
- `lua/viewer/*.lua`

如果使用 `lazy.nvim` 加载本地仓库, 可以直接写成:

```lua
{
  dir = "/home/ccls/github/viewer.nvim",
  name = "viewer.nvim",
  lazy = false,
  config = function()
    require("viewer").setup({
      probe_timeout_ms = 1000,
      auto_start = false,
      local_endpoint = { host = "127.0.0.1", port = 7357 },
      remote_endpoint = { host = "127.0.0.1", port = 7357 },
    })
  end,
}
```

如果希望在任意文件里按快捷键再加载插件, 推荐这样写:

```lua
{
  dir = "/home/ccls/github/viewer.nvim",
  name = "viewer.nvim",
  keys = {
    { "<leader>vp", "<cmd>ViewerPreview<cr>", desc = "viewer preview" },
    { "<leader>vt", "<cmd>ViewerToggle<cr>", desc = "viewer toggle" },
    { "<leader>vd", "<cmd>ViewerDocs<cr>", desc = "viewer docs" },
  },
  config = function()
    require("viewer").setup({
      probe_timeout_ms = 1000,
      auto_start = false,
      keymaps = {
        preview = false,
        toggle = false,
        interval = false,
        docs = false,
      },
    })
  end,
}
```

### 运行

1. 先启动 `nview`

```bash
./bin/nview
```

默认监听 `0.0.0.0:7357`, 需要时可以用 `-listen` 覆盖。

2. 在 Neovim 中打开 markdown 文件, 然后执行:

```vim
:ViewerPreview
```

3. 关闭预览:

```vim
:ViewerToggle
```

4. 查询当前光标词对应的离线文档:

```vim
:ViewerDocs
```

### 配置示例

```lua
require("viewer").setup({
  local_endpoint = { host = "127.0.0.1", port = 7357 },
  remote_endpoint = { host = "192.168.1.10", port = 7357 },
  probe_timeout_ms = 1000,
  auto_start = false,
  keymaps = {
    preview = "<leader>vp",
    toggle = "<leader>vt",
    interval = "<leader>vi",
    docs = "<leader>vd",
  },
})
```

如果不想要默认映射:

```lua
require("viewer").setup({
  keymaps = {
    preview = false,
    toggle = false,
    interval = false,
    docs = false,
  },
})
```
