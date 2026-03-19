package templates

import "embed"

// Pages holds all page templates embedded at compile time.
//
//go:embed pages/*.html
var Pages embed.FS

// Components holds all component templates embedded at compile time.
//
//go:embed components/*.html
var Components embed.FS
