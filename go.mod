module github.com/ccls/viewer.nvim

go 1.22

require (
	github.com/jchv/go-webview2 v0.0.0-20260205173254-56598839c808
	github.com/webview/webview_go v0.0.0-20240831120633-6173450d4dd6
	github.com/yuin/goldmark v1.8.2
)

require (
	github.com/jchv/go-winloader v0.0.0-20250406163304-c1995be93bd1 // indirect
	golang.org/x/sys v0.0.0-20210218145245-beda7e5e158e // indirect
)

replace github.com/jchv/go-webview2 => ./third_party/go-webview2
