package ik

import (
	"html/template"
	"sync"

	"github.com/sjc5/kit/pkg/safecache"
)

type runtime struct {
	initOnce sync.Once
	cache    runtimeCache
}

type runtimeCache struct {
	// FS
	uniFS     *safecache.Cache[UniversalFS]
	uniDirFS  *safecache.Cache[UniversalFS]
	publicFS  *safecache.Cache[UniversalFS]
	privateFS *safecache.Cache[UniversalFS]

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
			uniFS:     safecache.New(c.getInitialUniversalFS, GetIsDev),
			uniDirFS:  safecache.New(c.getInitialUniversalDirFS, GetIsDev),
			publicFS:  safecache.New(func() (UniversalFS, error) { return c.getFS(publicDir) }, GetIsDev),
			privateFS: safecache.New(func() (UniversalFS, error) { return c.getFS(privateDir) }, GetIsDev),

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
