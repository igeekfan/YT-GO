package main

import (
	_ "embed"
	"encoding/json"
)

//go:embed wails.json
var wailsJSON string

var WailsInfo struct {
	Info struct {
		ProductVersion string `json:"productVersion"`
	} `json:"info"`
}

var version = ""

func init() {
	_ = json.Unmarshal([]byte(wailsJSON), &WailsInfo)
}

func currentAppVersion() string {
	if version != "" {
		return version
	}
	return WailsInfo.Info.ProductVersion
}