需要实现两个东西,一个是viewer.nvim这个插件.一个是配套的nview这个小工具.具体描述见下文

## mvp版本

- 是一个nvim插件,支持nvim-0.10以上版本
- 配合一个外部能够渲染html的小工具nview使用,nview和nvim互相通信,可以通过网络进行通信,因此nvim和nview可以是分布在两台独立的机器上的c/s架构
- nview的目的是为了弥补终端的现代化渲染能力不足,例如图片和网页内容渲染等
- 当nvim失焦时候,nview自动隐藏.当获取焦点时,nview自动置顶显示.nview的大小根据nvim在终端中的窗口大小同步调整.nview能被用户拖动
- nview和nvim实现同步滚动内容,当nvim在编辑markdown文本时,开启该插件的markdown预览功能后(通过快捷键或者command都可以),配置好的远程nview或本地nview就开始渲染nvim中当前的markdown文档
- 当nvim检测到自己是通过ssh的环境中开启的,就优先尝试配置的远程nview,否则就优先使用本地nview.异步探测nview是否存在,如果未检测到,给出良好提示信息

## 编译与安装

### 依赖

- Go 1.22+
- Neovim 0.10+
- Linux: `wmctrl` 和桌面环境下可用的浏览器, 例如 `google-chrome` / `chromium`
- macOS: `Google Chrome`
- Windows: `Microsoft Edge` 或 `Google Chrome`

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

如果只想在 markdown 文件中加载:

```lua
{
  dir = "/home/ccls/github/viewer.nvim",
  name = "viewer.nvim",
  ft = { "markdown" },
  config = function()
    require("viewer").setup({
      probe_timeout_ms = 1000,
      auto_start = false,
    })
  end,
}
```

### 运行

1. 先启动 `nview`

```bash
./bin/nview
```

2. 在 Neovim 中打开 markdown 文件, 然后执行:

```vim
:ViewerPreview
```

3. 关闭预览:

```vim
:ViewerToggle
```

### 配置示例

```lua
require("viewer").setup({
  local_endpoint = { host = "127.0.0.1", port = 7357 },
  remote_endpoint = { host = "192.168.1.10", port = 7357 },
  probe_timeout_ms = 1000,
  auto_start = false,
})
```

## 后续

- 支持离线文档渲染和nvim交互
