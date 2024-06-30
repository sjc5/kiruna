package ik

import (
	"path/filepath"

	"github.com/sjc5/kit/pkg/colorlog"
)

const (
	internalDir          = "internal"
	publicDir            = "public"
	privateDir           = "private"
	staticDir            = "static"
	distKirunaDir        = "dist/kiruna"
	criticalCSSFile      = "critical.css"
	normalCSSFileRefFile = "normal_css_file_ref.txt"
)

var Log = colorlog.Log{Label: "Kiruna"}

func (c *Config) getCleanRootDir() string {
	return filepath.Clean(c.RootDir)
}
