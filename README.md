需要实现两个东西,一个是viewer.nvim这个插件.一个是配套的nview这个小工具.具体描述见下文

## mvp版本

- 是一个nvim插件,支持nvim-0.10以上版本
- 配合一个外部能够渲染html的小工具nview使用,nview和nvim互相通信,可以通过网络进行通信,因此nvim和nview可以是分布在两台独立的机器上的c/s架构
- nview的目的是为了弥补终端的现代化渲染能力不足,例如图片和网页内容渲染等
- 当nvim失焦时候,nview自动隐藏.当获取焦点时,nview自动置顶显示.nview的大小根据nvim在终端中的窗口大小同步调整.nview能被用户拖动
- nview和nvim实现同步滚动内容,当nvim在编辑markdown文本时,开启该插件的markdown预览功能后(通过快捷键或者command都可以),配置好的远程nview或本地nview就开始渲染nvim中当前的markdown文档
- 当nvim检测到自己是通过ssh的环境中开启的,就优先尝试配置的远程nview,否则就优先使用本地nview.异步探测nview是否存在,如果未检测到,给出良好提示信息


## 后续

- 支持离线文档渲染和nvim交互
