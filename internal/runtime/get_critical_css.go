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

var criticalCSS string
var criticalCSSIsCached bool
var criticalCSSDoesNotExist bool

func GetCriticalCSS(config *common.Config) string {
	if !common.KirunaEnv.GetIsDev() {
		if criticalCSSDoesNotExist {
			return ""
		}
		if criticalCSSIsCached {
			return criticalCSS
		}
	}
	FS, err := GetUniversalFS(config)
	if err != nil {
		util.Log.Errorf("error getting FS: %v", err)
		return ""
	}
	content, err := FS.ReadFile(filepath.Join("kiruna", "internal", "critical.css"))
	if err != nil {
		criticalCSSDoesNotExist = strings.HasSuffix(err.Error(), "no such file or directory")
		if !criticalCSSDoesNotExist {
			util.Log.Errorf("error reading critical CSS: %v", err)
		}
		return ""
	}
	criticalCSS = string(content)
	criticalCSSIsCached = true
	return criticalCSS
}

var criticalCSSStyleElement template.HTML
var criticalCSSStyleElementIsCached bool

func GetCriticalCSSStyleElement(config *common.Config) template.HTML {
	if !common.KirunaEnv.GetIsDev() {
		if criticalCSSDoesNotExist {
			return ""
		}
		if criticalCSSStyleElementIsCached {
			return criticalCSSStyleElement
		}
	}
	css := GetCriticalCSS(config)
	if criticalCSSDoesNotExist {
		return ""
	}
	criticalCSSStyleElement = template.HTML(fmt.Sprintf("<style id=\"%s\">%s</style>", CriticalCSSElementID, css))
	criticalCSSStyleElementIsCached = true
	return criticalCSSStyleElement
}
