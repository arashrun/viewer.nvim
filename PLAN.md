# viewer.nvim MVP 计划

## 目标

- 提供一个 Neovim 0.10+ 插件 `viewer.nvim`
- 提供一个配套工具 `nview`
- 先跑通 markdown 预览的远程通信、自动探测、窗口状态同步

## MVP 范围

1. 插件初始化与配置合并
2. `:ViewerPreview` / `:ViewerToggle` 命令
3. 本地与远程 `nview` 地址探测
4. TCP 连接与 JSONL 协议
5. markdown 缓冲区内容同步
6. 焦点、尺寸、滚动位置同步
7. `nview` 侧先提供一个可运行的消息接收器，后续再接入真正的 HTML 渲染窗口

## 里程碑

1. `lua/viewer/*` 基础模块完成
2. `plugin/viewer.lua` 注册命令和自动命令
3. `cmd/nview/main.go` 提供最小可运行程序
4. 再补真实渲染、拖动和置顶逻辑

## 后续

- 离线文档渲染
- 非 markdown 内容的通用网页/图片渲染
- 更完整的窗口生命周期控制
