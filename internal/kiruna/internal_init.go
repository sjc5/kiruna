package ik

import "path/filepath"

func (c *Config) Private_CommonInitOnce_OnlyCallInNewFunc() {
	c.commonInitOnce.Do(func() {
		c.validateConfig()

		c.initializedWithNew = true

		c.cleanSrcDirs = CleanSrcDirs{
			PrivateStatic: filepath.Clean(c.PrivateStaticDir),
			PublicStatic:  filepath.Clean(c.PublicStaticDir),
			Styles:        filepath.Clean(c.StylesDir),
			Dist:          filepath.Clean(c.DistDir),
		}

		c.__dist = toDistLayout(c.cleanSrcDirs.Dist)
	})
}
