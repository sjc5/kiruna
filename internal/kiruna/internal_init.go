package ik

import "path/filepath"

func (c *Config) Private_CommonInitOnce_OnlyCallInNewFunc() {
	c.commonInitOnce.Do(func() {
		c.validateConfig()

		c.initializedWithNew = true

		c.cleanSources = CleanSources{
			Dist:            filepath.Clean(c.DistDir),
			PrivateStatic:   filepath.Clean(c.PrivateStaticDir),
			PublicStatic:    filepath.Clean(c.PublicStaticDir),
			CriticalCSSFile: filepath.Clean(c.CriticalCSSFile),
			NormalCSSFile:   filepath.Clean(c.NormalCSSFile),
		}

		c.__dist = toDistLayout(c.cleanSources.Dist)
	})
}
