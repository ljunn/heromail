package web

import "embed"

// Files 是用户端和管理员端页面的内嵌文件系统。
//
//go:embed static/*
var Files embed.FS
