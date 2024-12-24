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
	distKirunaDir            = "kiruna"
	criticalCSSFile          = "critical.css"
	normalCSSFileRefFile     = "normal_css_file_ref.txt"
	publicFileMapFileRefFile = "public_file_map_file_ref.txt"
	binOutPath               = "bin/main"
	goEmbedFixerFile         = "x"
)

type cleanDirs struct {
	PrivateStatic string
	PublicStatic  string
	Styles        string
	Dist          string
}

func (c *Config) getCleanDirs() cleanDirs {
	return cleanDirs{
		PrivateStatic: filepath.Clean(c.PrivateStaticDir),
		PublicStatic:  filepath.Clean(c.PublicStaticDir),
		Styles:        filepath.Clean(c.StylesDir),
		Dist:          filepath.Clean(c.DistDir),
	}
}

func (c *Config) getCleanWatchRoot() string {
	return filepath.Clean(c.DevConfig.WatchRoot)
}
