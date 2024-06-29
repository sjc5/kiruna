package runtime

import (
	"fmt"
	"html/template"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/util"
	"github.com/sjc5/kit/pkg/typed"
)

const CriticalCSSElementID = "__critical-css"

type criticalCSSStatus struct {
	mu              sync.RWMutex
	codeStr         string
	noSuchFile      bool
	styleEl         template.HTML
	styleElIsCached bool
}

var cacheMap = typed.SyncMap[*common.Config, *criticalCSSStatus]{}

func GetCriticalCSS(config *common.Config) string {
	// If cache hit and PROD, return hit
	if hit, isCached := cacheMap.Load(config); isCached && !common.KirunaEnv.GetIsDev() {
		hit.mu.RLock()
		defer hit.mu.RUnlock()
		if hit.noSuchFile {
			return ""
		}
		return hit.codeStr
	}

	// Instantiate cache or get existing
	cachedStatus, _ := cacheMap.LoadOrStore(config, &criticalCSSStatus{})

	// Get FS
	fs, err := GetUniversalFS(config)
	if err != nil {
		util.Log.Errorf("error getting FS: %v", err)
		return ""
	}

	// Read critical CSS
	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	content, err := fs.ReadFile(filepath.Join("internal", "critical.css"))
	if err != nil {
		// Check if the error is a non-existent file
		noSuchFile := strings.HasSuffix(err.Error(), "no such file or directory")

		cachedStatus.mu.Lock()
		cachedStatus.noSuchFile = noSuchFile // Set noSuchFile flag in cache
		cachedStatus.mu.Unlock()

		// if the error was something other than a non-existent file, log it
		if !noSuchFile {
			util.Log.Errorf("error reading critical CSS: %v", err)
		}
		return ""
	}

	criticalCSS := string(content)

	cachedStatus.mu.Lock()
	cachedStatus.codeStr = criticalCSS // Cache the critical CSS
	cachedStatus.mu.Unlock()

	return criticalCSS
}

func GetCriticalCSSStyleElement(config *common.Config) template.HTML {
	// If cache hit and PROD, return hit
	if hit, isCached := cacheMap.Load(config); isCached && !common.KirunaEnv.GetIsDev() {
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
	css := GetCriticalCSS(config)
	cached, _ := cacheMap.Load(config)

	cached.mu.RLock()
	noSuchFile := cached.noSuchFile
	cached.mu.RUnlock()

	// At this point, noSuchFile will have been set by GetCriticalCSS call above
	if noSuchFile {
		return ""
	}

	// Create style element
	el := template.HTML(fmt.Sprintf("<style id=\"%s\">%s</style>", CriticalCSSElementID, css))

	cached.mu.Lock()
	cached.styleEl = el           // Cache the element
	cached.styleElIsCached = true // Set element as cached
	cached.mu.Unlock()

	return el
}
