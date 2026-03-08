//go:build embed_ui

package evidrabenchmark

import (
	"embed"
	"io/fs"
)

//go:embed all:ui/dist
var uiDistRaw embed.FS

func init() {
	sub, err := fs.Sub(uiDistRaw, "ui/dist")
	if err != nil {
		panic("uiembed: " + err.Error())
	}
	UIDistFS = sub
}
