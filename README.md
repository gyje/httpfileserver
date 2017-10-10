# httpfileserver
极简文件服务器，默认后台运行，配合内网穿透可以实现访问运行目录下的文件,默认80端口，编译命令：`go build -ldflags "-H windowsgui -s -w" serve.go`可以实现后台运行，建议加个upx的壳，可以进一步减小体积。
