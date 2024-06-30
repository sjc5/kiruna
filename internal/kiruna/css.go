package ik

import (
	"html/template"
	"path/filepath"
	"strings"
	"sync"
)

const (
	CriticalCSSElementID = "__critical-css"
	StyleSheetElementID  = "__normal-css"
)

func (c *Config) GetCriticalCSSStyleElement() template.HTML {
	// If cache hit and PROD, return hit
	if hit, isCached := cache.criticalCSS.Load(c); isCached && !KirunaEnv.GetIsDev() {
		hit.mu.RLock()
		defer hit.mu.RUnlock()
		if hit.noSuchFile {
			return ""
		}
		if hit.styleElIsCached {
			return hit.styleEl
		}
	}

	// Get critical CSS
	css := c.GetCriticalCSS()
	cached, _ := cache.criticalCSS.Load(c)

	cached.mu.RLock()
	noSuchFile := cached.noSuchFile
	cached.mu.RUnlock()

	// At this point, noSuchFile will have been set by GetCriticalCSS call above
	if noSuchFile {
		return ""
	}

	// Create style element
	var sb strings.Builder
	sb.WriteString(`<style id="`)
	sb.WriteString(CriticalCSSElementID)
	sb.WriteString(`">`)
	sb.WriteString(css)
	sb.WriteString("</style>")
	el := template.HTML(sb.String())

	cached.mu.Lock()
	defer cached.mu.Unlock()
	cached.styleEl = el           // Cache the element
	cached.styleElIsCached = true // Set element as cached

	return el
}

func (c *Config) GetStyleSheetLinkElement() template.HTML {
	if hit, isCached := cache.styleSheetElement.Load(c); isCached && !KirunaEnv.GetIsDev() {
		return hit
	}

	url := c.GetStyleSheetURL()
	if url == "" {
		cache.styleSheetElement.Store(c, "") // Cache the empty string
		return ""
	}

	var sb strings.Builder
	sb.WriteString(`<link rel="stylesheet" href="`)
	sb.WriteString(url)
	sb.WriteString(`" id="`)
	sb.WriteString(StyleSheetElementID)
	sb.WriteString(`" />`)
	el := template.HTML(sb.String())

	cache.styleSheetElement.Store(c, el) // Cache the element
	return el
}

func (c *Config) GetStyleSheetURL() string {
	if hit, isCached := cache.styleSheetURL.Load(c); isCached && !KirunaEnv.GetIsDev() {
		return hit
	}

	fs, err := c.GetUniversalFS()
	if err != nil {
		c.Logger.Errorf("error getting FS: %v", err)
		return ""
	}

	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	content, err := fs.ReadFile(filepath.Join(internalDir, normalCSSFileRefFile))
	if err != nil {
		c.Logger.Errorf("error reading normal CSS URL: %v", err)
		return ""
	}

	url := "/" + publicDir + "/" + string(content)
	cache.styleSheetURL.Store(c, url) // Cache the URL
	return url
}

type criticalCSSStatus struct {
	mu              sync.RWMutex
	codeStr         string
	noSuchFile      bool
	styleEl         template.HTML
	styleElIsCached bool
}

func (c *Config) GetCriticalCSS() string {
	// If cache hit and PROD, return hit
	if hit, isCached := cache.criticalCSS.Load(c); isCached && !KirunaEnv.GetIsDev() {
		hit.mu.RLock()
		defer hit.mu.RUnlock()
		if hit.noSuchFile {
			return ""
		}
		return hit.codeStr
	}

	// Instantiate cache or get existing
	cachedStatus, _ := cache.criticalCSS.LoadOrStore(c, &criticalCSSStatus{})

	// Get FS
	fs, err := c.GetUniversalFS()
	if err != nil {
		c.Logger.Errorf("error getting FS: %v", err)
		return ""
	}

	// Read critical CSS
	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	content, err := fs.ReadFile(filepath.Join(internalDir, criticalCSSFile))
	cachedStatus.mu.Lock()
	defer cachedStatus.mu.Unlock()

	if err != nil {
		// Check if the error is a non-existent file, and set the noSuchFile flag in the cache
		cachedStatus.noSuchFile = strings.HasSuffix(err.Error(), "no such file or directory")

		// if the error was something other than a non-existent file, log it
		if !cachedStatus.noSuchFile {
			c.Logger.Errorf("error reading critical CSS: %v", err)
		}
		return ""
	}

	criticalCSS := string(content)
	cachedStatus.codeStr = criticalCSS // Cache the critical CSS

	return criticalCSS
}

func naiveCSSMinify(content string) string {
	return strings.Join(strings.Fields(content), " ")
}
