package ik

import (
	"html/template"
	"io/fs"
	"sync"

	"github.com/sjc5/kit/pkg/safecache"
)

type runtime struct {
	initOnce sync.Once
	cache    runtimeCache
}

type runtimeCache struct {
	// FS
	baseFS    *safecache.Cache[fs.FS]
	baseDirFS *safecache.Cache[fs.FS]
	publicFS  *safecache.Cache[fs.FS]
	privateFS *safecache.Cache[fs.FS]

	// CSS
	styleSheetLinkElement *safecache.Cache[*template.HTML]
	styleSheetURL         *safecache.Cache[string]
	criticalCSS           *safecache.Cache[*criticalCSSStatus]

	// Public URLs
	publicFileMapFromGob *safecache.Cache[map[string]string]
	publicFileMapURL     *safecache.Cache[string]
	publicFileMapDetails *safecache.Cache[*publicFileMapDetails]
	publicURLs           *safecache.CacheMap[string, string, string]
}

func (c *Config) RuntimeInitOnce() {
	c.runtime.initOnce.Do(func() {
		// cache
		c.cache = runtimeCache{
			// FS
			baseFS:    safecache.New(c.getInitialBaseFS, GetIsDev),
			baseDirFS: safecache.New(c.getInitialBaseDirFS, GetIsDev),
			publicFS:  safecache.New(func() (fs.FS, error) { return c.getSubFS(publicDir) }, GetIsDev),
			privateFS: safecache.New(func() (fs.FS, error) { return c.getSubFS(privateDir) }, GetIsDev),

			// CSS
			styleSheetLinkElement: safecache.New(c.getInitialStyleSheetLinkElement, GetIsDev),
			styleSheetURL:         safecache.New(c.getInitialStyleSheetURL, GetIsDev),
			criticalCSS:           safecache.New(c.getInitialCriticalCSSStatus, GetIsDev),

			// Public URLs
			publicFileMapFromGob: safecache.New(c.getInitialPublicFileMapFromGobRuntime, GetIsDev),
			publicFileMapURL:     safecache.New(c.getInitialPublicFileMapURL, GetIsDev),
			publicFileMapDetails: safecache.New(c.getInitialPublicFileMapDetails, GetIsDev),
			publicURLs: safecache.NewMap(c.getInitialPublicURL, publicURLsKeyMaker, func(string) bool {
				return GetIsDev()
			}),
		}
	})
}
