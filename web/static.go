package web

import "embed"

//go:embed public
var StaticFS embed.FS
