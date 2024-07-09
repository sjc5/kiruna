package ik

import (
	"path/filepath"
)

const (
	internalDir              = "internal"
	publicDir                = "public"
	publicInternalDir        = "kiruna_internal__"
	kirunaDir                = "kiruna"
	privateDir               = "private"
	staticDir                = "static"
	stylesDir                = "styles"
	distKirunaDir            = "dist/kiruna"
	criticalCSSFile          = "critical.css"
	normalCSSFileRefFile     = "normal_css_file_ref.txt"
	publicFileMapFileRefFile = "public_file_map_file_ref.txt"
	pidFile                  = "__kiruna_dev.pid"
	binOutPath               = "dist/bin/main"
	goEmbedFixerFile         = "x"
)

func (c *Config) getCleanRootDir() string {
	return filepath.Clean(c.RootDir)
}
