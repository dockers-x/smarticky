package web

import "embed"

//go:embed all:static templates
var Assets embed.FS
