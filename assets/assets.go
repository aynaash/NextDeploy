// Package assets is aws embed binary
package assets

import "embed"

//go:embed lambda/*
var LambdaBinaries embed.FS
