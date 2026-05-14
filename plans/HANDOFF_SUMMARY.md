# nview / viewer.nvim 交接摘要

## 目标

- `viewer.nvim` 是 Neovim 插件。
- `nview` 是独立桌面应用，不是浏览器窗口，也不是 Electron/Tauri 方向。
- 核心职责是通过 TCP JSON 接收状态，渲染 Markdown，并控制窗口显示、隐藏、聚焦。
- 需要继续保证 Linux 构建和 Windows 64 位交叉编译可用。

## 当前结论

- 方案已确认继续走 Go。
- `nview` 的窗口 geometry 不再由 `nvim` 的 `preview` / `viewport` 消息同步控制。
- 当前改成“本地窗口状态记忆”方案：窗口边界、可见性、置顶、聚焦状态都进入 `WindowState`。
- Windows 上已实现“启动完全隐藏，连接后显示，显示时置顶”。
- `viewer.nvim` 已支持 `nview` 断开后自动重连，`nview` 重启后会自动恢复连接和预览。
- 之前已经提交过一次整理，提交号是 `b354400`，提交信息是 `refactor(nview): 统一窗口抽象并修复构建`。

## 当前实现

- `cmd/nview/main.go`
  - 启动时支持 `-state-file`。
  - 启动时会尝试读取窗口状态文件，再与默认状态合并。
  - `preview` 只更新内容，不再驱动窗口 geometry。
  - `viewport` 只更新状态，不再触发 `Resize()`。
  - `focus=true` 时显式 `Show()`，`focus=false` 时显式 `Hide()`。
  - 退出时会保存当前窗口状态。
- `cmd/nview/window_state.go`
  - 定义了 `WindowBounds` 和 `WindowState`。
  - 默认窗口大小目前是 `860x620`。
  - 默认状态里 `Visible`、`Focused` 关闭，`TopMost` 打开。
- `cmd/nview/window_persist.go`
  - 窗口状态保存在用户配置目录下，默认路径类似：
    - `~/.config/viewer.nvim/nview-window.json`
- `cmd/nview/window_common.go`
  - `WindowController` 现在持有 `state`。
  - `Attach()` 后会按状态应用 bounds / topmost / show / hide。
  - `Hide()` 和 `Stop()` 前会记住当前窗口边界并写回 state。
  - `OnWindowBoundsChanged()` 由窗口拖动结束事件触发，用来立即保存 geometry。
- `cmd/nview/window_windows.go`
  - Windows 侧通过 `user32.dll` 做原生控制。
  - 目前实现了 `ShowWindow`、`SetWindowPos`、`GetWindowRect`、`SetFocus`。
  - 保存边界时用的是外框矩形，不再依赖客户端区域。
  - 显示时使用 topmost 置顶策略，并且已修正 `SWP_NOZORDER` 导致的失效问题。
- `cmd/nview/window_linux.go`
  - Linux 侧保留 `SetBounds()`，但可见性、置顶、聚焦多数是空实现或弱实现。
- `lua/viewer/init.lua`
  - 负责向 `nview` 发 `focus` 和同步请求。
  - 已加入 `nview` 断开后的自动重连。
  - 已修复初始化阶段若干 Lua 局部函数顺序问题。
- `lua/viewer/transport.lua`
  - 现在能感知 TCP 断开，并通过 `on_close` 回调触发重连。
- `plugin/viewer.lua`
  - 增加了 `require("viewer")` 失败时的明确提示，方便排查最小安装缺失 `lua/viewer/*.lua` 的情况。

## 已知现状

- Windows 上的窗口问题，已经从“被 `nvim` 强制同步”转成“本地状态恢复 / 原生窗口行为”问题。
- 现在已验证：
  - 启动时可以完全隐藏
  - 连接后可以显示
  - 显示时可以置顶
  - geometry 会在拖动结束时和隐藏/退出时写回 state
- 如果看起来还是没记住 geometry，优先检查 state 文件是否真的被更新，以及用户拖动是否触发了 `WM_EXITSIZEMOVE`。
- 如果 `nview` 重启，`viewer.nvim` 应该自动重连；如果没有重连，优先检查 TCP 监听和端口是否一致。

## 待继续确认

1. Windows 上手动拖动结束后，state 文件里的 `bounds` 是否立即更新。
2. `nview` 重启后，`viewer.nvim` 的自动重连是否稳定恢复当前 markdown 预览。
3. Linux 侧的窗口控制接口是否需要继续补齐，还是保持最小实现即可。
4. 入口提示是否足够清楚，能否避免“最小插件漏掉 `lua/viewer/*.lua`”这类安装问题。

## 建议下一步

1. 继续在 Windows 上验证“拖动结束立即保存 geometry”这条链路。
2. 继续验证 `nview` 重启后的自动重连是否会恢复当前 markdown 预览。
3. 如果要做发布，先确认 Windows 最小安装包里必须包含 `plugin/viewer.lua` 和 `lua/viewer/*.lua`。
