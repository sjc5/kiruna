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
			uniFS:     safecache.New(c.getInitialUniversalFS, getIsDev),
			uniDirFS:  safecache.New(c.getInitialUniversalDirFS, getIsDev),
			publicFS:  safecache.New(func() (UniversalFS, error) { return c.getFS(publicDir) }, getIsDev),
			privateFS: safecache.New(func() (UniversalFS, error) { return c.getFS(privateDir) }, getIsDev),

			// CSS
			styleSheetLinkElement: safecache.New(c.getInitialStyleSheetLinkElement, getIsDev),
			styleSheetURL:         safecache.New(c.getInitialStyleSheetURL, getIsDev),
			criticalCSS:           safecache.New(c.getInitialCriticalCSSStatus, getIsDev),

			// Public URLs
			publicFileMapFromGob: safecache.New(c.getInitialPublicFileMapFromGob, getIsDev),
			publicFileMapURL:     safecache.New(c.getInitialPublicFileMapURL, getIsDev),
			publicFileMapDetails: safecache.New(c.getInitialPublicFileMapDetails, getIsDev),
			publicURLs:           safecache.NewMap(c.getInitialPublicURL, publicURLsKeyMaker, func(string) bool { return getIsDev() }),
		}
	})
}
