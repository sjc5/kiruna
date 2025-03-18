package ik

import "path/filepath"

func (c *Config) Private_CommonInitOnce_OnlyCallInNewFunc() {
	c.commonInitOnce.Do(func() {
		c.validateConfig()

		c.initializedWithNew = true

		c.cleanSources = CleanSources{
			Dist:             filepath.Clean(c.DistDir),
			PrivateStatic:    filepath.Clean(c.PrivateStaticDir),
			PublicStatic:     filepath.Clean(c.PublicStaticDir),
			CriticalCSSEntry: filepath.Clean(c.CriticalCSSEntry),
			NormalCSSEntry:   filepath.Clean(c.NormalCSSEntry),
		}

		c.__dist = toDistLayout(c.cleanSources.Dist)
	})
}
