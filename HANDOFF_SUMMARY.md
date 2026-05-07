# nview / viewer.nvim 交接摘要

## 目标

- `viewer.nvim` 是 Neovim 插件。
- `nview` 是独立桌面应用，不是浏览器窗口，也不是 Electron/Tauri 方向。
- 核心职责是通过 TCP JSON 接收状态，渲染 Markdown，并控制窗口显示、隐藏、聚焦。
- 需要继续保证 Linux 构建和 Windows 64 位交叉编译可用。

## 当前结论

- 方案已确认继续走 Go。
- `nview` 的窗口尺寸不再由 `nvim` 的 `preview` / `viewport` 消息同步控制。
- 当前改成“本地窗口状态记忆”方案：窗口边界、可见性、置顶、聚焦状态都进入 `WindowState`。
- 之前已经提交过一次整理，提交号是 `b354400`，提交信息是 `refactor(nview): 统一窗口抽象并修复构建`。

## 当前实现

- `cmd/nview/main.go`
  - 启动时支持 `-state-file`。
  - 启动时会尝试读取窗口状态文件，再与默认状态合并。
  - `preview` 只更新内容并 `Show()`。
  - `viewport` 只更新状态，不再触发 `Resize()`。
  - `focus` 事件改为显式 `Show()` / `Hide()`。
  - 退出时会保存当前窗口状态。
- `cmd/nview/window_state.go`
  - 定义了 `WindowBounds` 和 `WindowState`。
  - 默认窗口大小目前是 `860x620`。
  - 默认状态里 `Visible`、`Focused`、`TopMost` 都是开启的。
- `cmd/nview/window_persist.go`
  - 窗口状态保存在用户配置目录下，默认路径类似：
    - `~/.config/viewer.nvim/nview-window.json`
- `cmd/nview/window_common.go`
  - `WindowController` 现在持有 `state`。
  - `Attach()` 后会按状态应用 bounds / topmost / show / hide / focus。
  - `Stop()` 前会先记住当前窗口边界。
- `cmd/nview/window_windows.go`
  - Windows 侧通过 `user32.dll` 做原生控制。
  - 目前实现了 `ShowWindow`、`SetWindowPos`、`GetWindowRect`、`SetFocus`。
  - 保存边界时用的是外框矩形，不再依赖客户端区域。
- `cmd/nview/window_linux.go`
  - Linux 侧保留 `SetBounds()`，但可见性、置顶、聚焦多数是空实现或弱实现。
- `lua/viewer/init.lua`
  - 只负责向 `nview` 发 `focus` 和同步请求。
  - 不再承担窗口尺寸同步逻辑。

## 已知现状

- Windows 上的窗口尺寸问题，已经从“被 `nvim` 强制同步”转成“本地状态恢复 / 原生窗口行为”问题。
- 如果窗口启动时只剩标题栏，优先怀疑的是 Windows 侧窗口边界恢复、原生显示时机、或保存的边界值无效。
- 如果手动调大后又在切回 `nvim` 时变成很小，说明还有一层状态恢复或窗口展示流程需要继续排查，但不再是 `viewport` 驱动的自动缩放。

## 待继续确认

1. Windows 启动后是否稳定恢复到有效边界。
2. 手动拖拽后的边界是否能被正确保存并在下次启动时恢复。
3. `Show()` / `Hide()` / `Focus()` 在 Windows 上是否符合预期。
4. Linux 侧的窗口控制接口是否需要补齐，还是保持最小实现即可。

## 建议下一步

1. 继续在 Windows 上验证“启动默认大小、手动调整、切回 nvim、再次聚焦”这条链路。
2. 如果仍然出现标题栏-only 或变回 0 大小，优先检查 `applyState()`、`SetBounds()` 和 state 文件内容。
3. 视需要再决定是否补一轮更明确的窗口状态日志，方便定位 Windows 行为差异。
