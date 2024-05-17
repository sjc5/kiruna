package runtime

import (
	"fmt"
	"html/template"
	"path/filepath"

	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/util"
)

const StyleSheetElementID = "__normal-css"

var styleSheetURLCacheMap = make(map[*common.Config]string)

func GetStyleSheetURL(config *common.Config) string {
	cachedURL, isCached := styleSheetURLCacheMap[config]
	if isCached {
		return cachedURL
	}

	fs, err := GetUniversalFS(config)
	if err != nil {
		util.Log.Errorf("error getting FS: %v", err)
		return ""
	}

	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	content, err := fs.ReadFile(filepath.Join("internal", "normal_css_file_ref.txt"))
	if err != nil {
		util.Log.Errorf("error reading normal CSS URL: %v", err)
		return ""
	}

	url := "/public/" + string(content)
	styleSheetURLCacheMap[config] = url // Cache the URL
	return url
}

var styleSheetElementCacheMap = make(map[*common.Config]template.HTML)

func GetStyleSheetLinkElement(config *common.Config) template.HTML {
	cachedEl, isCached := styleSheetElementCacheMap[config]
	if isCached {
		return cachedEl
	}

	url := GetStyleSheetURL(config)
	if url == "" {
		styleSheetElementCacheMap[config] = "" // Cache the empty string
		return ""
	}

	el := template.HTML(fmt.Sprintf(
		"<link rel=\"stylesheet\" href=\"%s\" id=\"%s\" />", url, StyleSheetElementID,
	))
	styleSheetElementCacheMap[config] = el // Cache the element
	return el
}
