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
	publicURLs           *safecache.CacheMap[string, string, string]
}

func (c *Config) RuntimeInitOnce() {
	c.runtime.initOnce.Do(func() {
		// cache
		c.cache = runtimeCache{
			// FS
			uniFS:     safecache.New(c.getInitialUniversalFS, nil),
			uniDirFS:  safecache.New(c.getInitialUniversalDirFS, nil),
			publicFS:  safecache.New(func() (UniversalFS, error) { return c.getFS(publicDir) }, nil),
			privateFS: safecache.New(func() (UniversalFS, error) { return c.getFS(privateDir) }, nil),

			// CSS
			styleSheetLinkElement: safecache.New(c.getInitialStyleSheetLinkElement, getIsDev),
			styleSheetURL:         safecache.New(c.getInitialStyleSheetURL, getIsDev),
			criticalCSS:           safecache.New(c.getInitialCriticalCSSStatus, getIsDev),

			// Public URLs
			publicFileMapFromGob: safecache.New(c.getInitialPublicFileMapFromGob, nil),
			publicFileMapURL:     safecache.New(c.getInitialPublicFileMapURL, getIsDev),
			publicURLs:           safecache.NewMap(c.getInitialPublicURL, publicURLsKeyMaker, nil),
		}
	})
}
