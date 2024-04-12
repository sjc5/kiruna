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

// __TODO cache this?

func GetCriticalCSS(config *common.Config) string {
	FS, err := GetUniversalFS(config)
	if err != nil {
		util.Log.Errorf("error getting FS: %v", err)
		return ""
	}
	content, err := FS.ReadFile(filepath.Join("kiruna", "internal", "critical.css"))
	if err != nil {
		if !strings.HasSuffix(err.Error(), "no such file or directory") {
			util.Log.Errorf("error reading critical CSS: %v", err)
		}
		return ""
	}
	return string(content)
}

func GetCriticalCSSStyleElement(config *common.Config) template.HTML {
	css := GetCriticalCSS(config)
	if css == "" {
		return ""
	}
	return template.HTML(fmt.Sprintf("<style id=\"%s\">%s</style>", CriticalCSSElementID, css))
}
