package main

import "embed"

//go:embed application.yaml
var embeddedConfig []byte

//go:embed static/static
var embeddedStatic embed.FS
