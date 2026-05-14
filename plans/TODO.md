需要实现两个东西,一个是viewer.nvim这个插件.一个是配套的nview这个桌面端小工具.具体描述见下文

## mvp初始版本

- 是一个nvim插件,支持nvim-0.10以上版本
- 配合一个外部能够渲染html的小工具nview使用,nview和nvim互相通信,可以通过网络进行通信,因此nvim和nview可以是分布在两台独立的机器上的c/s架构
- nview的目的是为了弥补终端的现代化渲染能力不足,例如图片和网页内容渲染等
- nview需要跨平台,目前主要聚焦在windows平台
- 当nvim失焦时候,nview自动隐藏.当获取焦点时,nview自动置顶显示.nview能被用户拖动,放缩
- nview和nvim实现同步滚动内容,当nvim在编辑markdown文本时,开启该插件的markdown预览功能后(通过快捷键或者command都可以),配置好的远程nview或本地nview就开始渲染nvim中当前的markdown文档
- 当nvim检测到自己是通过ssh的环境中开启的,就优先尝试配置的远程nview,否则就优先使用本地nview.异步探测nview是否存在,如果未检测到,给出良好提示信息



## v1.1

- [x] 插件支持配置自定义自动隐藏的时间间隔(目前固定是3s)
- [x] nview当前连接上的不同客户端支持不同的geometry,实现不同nvim实例活跃的时候nview可以显示在不同位置和大小.新连接上的nview客户端使用state配置文件中的geometry. 
- [x] 支持markdown文档中多媒体素材的渲染,例如图片,因为是跨网络的架构,你最好先计划一下如何实现
- [x] 能实现nview中高亮当前nvim光标所在行吗


## v1.2

- [x] 支持离线API文档渲染和nvim交互查询,并且需要兼容docset这种zeal或dash兼容的格式
