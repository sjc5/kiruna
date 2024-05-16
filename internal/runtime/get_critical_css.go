package runtime

import (
	"fmt"
	"html/template"
	"path/filepath"
	"strings"

	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/util"
)

const CriticalCSSElementID = "__critical-css"

type criticalCSSStatus struct {
	codeStr         string
	noSuchFile      bool
	styleEl         template.HTML
	styleElIsCached bool
}

var cacheMap = make(map[*common.Config]*criticalCSSStatus)

func GetCriticalCSS(config *common.Config) string {
	// If cache hit and PROD, return hit
	if hit, isCached := cacheMap[config]; isCached && !common.KirunaEnv.GetIsDev() {
		if hit.noSuchFile {
			return ""
		}
		return hit.codeStr
	}

	// Instantiate cache
	cacheMap[config] = &criticalCSSStatus{}

	// Get FS
	fs, err := GetUniversalFS(config)
	if err != nil {
		util.Log.Errorf("error getting FS: %v", err)
		return ""
	}

	// Read critical CSS
	content, err := fs.ReadFile(filepath.Join("kiruna", "internal", "critical.css"))
	if err != nil {
		// Check if the error is a non-existent file
		noSuchFile := strings.HasSuffix(err.Error(), "no such file or directory")

		cacheMap[config].noSuchFile = noSuchFile // Set noSuchFile flag in cache

		// if the error was something other than a non-existent file, log it
		if !noSuchFile {
			util.Log.Errorf("error reading critical CSS: %v", err)
		}

		return ""
	}

	criticalCSS := string(content)
	cacheMap[config].codeStr = criticalCSS // Cache the critical CSS
	return criticalCSS
}

func GetCriticalCSSStyleElement(config *common.Config) template.HTML {
	// If cache hit and PROD, return hit
	if hit, isCached := cacheMap[config]; isCached && !common.KirunaEnv.GetIsDev() {
		if hit.noSuchFile {
			return ""
		}
		if hit.styleElIsCached {
			return hit.styleEl
		}
	}

	// Get critical CSS
	css := GetCriticalCSS(config)

	// At this point, noSuchFile will have been set by GetCriticalCSS call above
	if cacheMap[config].noSuchFile {
		return ""
	}

	// Create style element
	el := template.HTML(fmt.Sprintf("<style id=\"%s\">%s</style>", CriticalCSSElementID, css))
	cacheMap[config].styleEl = el           // Cache the element
	cacheMap[config].styleElIsCached = true // Set element as cached
	return el
}
