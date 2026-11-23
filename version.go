package main

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var defaultVersion string

var version = ""

func currentAppVersion() string {
	if version != "" {
		return version
	}
	return strings.TrimSpace(defaultVersion)
}
