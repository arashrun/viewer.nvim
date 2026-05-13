# viewer.nvim v1.1 计划

## 目标

- 在现有 MVP 基础上增强窗口体验、媒体渲染能力和光标态同步。
- 保持 Neovim 0.10+ 与 Windows 优先的实现方向不变。
- 继续沿用 `viewer.nvim` + `nview` 的 TCP JSONL 架构，不推倒重做。

## 已确认范围

1. 插件支持配置自定义自动隐藏时间间隔，替代当前固定 3s。
2. `nview` 支持按 `nvim` 实例会话记忆不同的 geometry。
3. Markdown 中的图片支持相对路径自动解析。
4. `nview` 中支持高亮当前 `nvim` 光标所在行。

## 设计约束

- 图片能力优先支持 Markdown 场景，不先扩展到完整通用网页资源管线。
- 相对路径解析以当前预览文档所在路径为基准。
- 多客户端 geometry 记忆的 key 先按 `nvim` 实例会话维度设计，不按文件维度拆分。
- 现有自动探测、本地/远程连接、重连逻辑继续保留。

## 技术方案

### 1. 自定义自动隐藏间隔

- 在 `lua/viewer/config.lua` 增加 `auto_hide_ms` 配置项。
- `viewer.nvim` 侧把该值传递到重连和状态同步逻辑中。
- `nview` 侧用该配置值替换当前固定的 3 秒 inactivity 判定。

### 2. 按会话记忆 geometry

- 扩展客户端标识，增加稳定的 `session_id` 概念。
- `nview` 维护 `session_id -> WindowState` 的映射。
- 当某个会话重新连接时，优先恢复该会话上次保存的 bounds。
- 新会话首次连接时，回退到全局默认 state 文件中的 geometry。

### 3. 图片相对路径解析

- `viewer.nvim` 在发送 markdown preview 时，继续发送当前文档路径。
- `nview` 根据 `path` 获取 markdown 文件所在目录。
- 渲染阶段扫描 markdown 图片引用，识别相对路径。
- 相对路径统一解析为绝对路径或可加载资源，然后交给渲染层显示。
- 先覆盖常规 Markdown 图片语法，后续再考虑更完整的资源协议。

### 4. 当前行高亮

- `viewer.nvim` 已经具备 cursor 信息同步基础，在协议中补充或固化当前行号。
- `nview` 维护“当前活动行”的渲染状态。
- 渲染层根据活动行更新高亮样式，确保滚动时同步更新。

## 里程碑

1. 配置和协议扩展完成
2. 会话级 geometry 记忆完成
3. 图片相对路径解析完成
4. 当前行高亮完成
5. Windows 产物编译并验证

## 验收条件

- `auto_hide_ms` 可配置且默认行为不回退。
- 同一台机器上多个 `nvim` 实例可以分别记忆 `nview` 的 geometry。
- Markdown 中的相对路径图片可以正常解析并显示。
- 当前光标所在行在 `nview` 里可见高亮。
- Windows 下可以成功编译生成 `./bin/nview.exe`。

## 风险点

- 图片相对路径解析会牵涉资源基准目录和远程场景，需避免误读工作目录。
- 当前行高亮需要明确 markdown 源行与渲染后 DOM 的映射方式。
- 会话级 geometry 的 key 生成如果不稳定，可能导致恢复错位或覆盖。

