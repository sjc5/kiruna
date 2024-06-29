package runtime

import (
	"fmt"
	"html/template"
	"path/filepath"

	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/util"
	"github.com/sjc5/kit/pkg/typed"
)

const StyleSheetElementID = "__normal-css"

var styleSheetURLCacheMap = typed.SyncMap[*common.Config, string]{}

func GetStyleSheetURL(config *common.Config) string {
	if hit, isCached := styleSheetURLCacheMap.Load(config); isCached && !common.KirunaEnv.GetIsDev() {
		return hit
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
	styleSheetURLCacheMap.Store(config, url) // Cache the URL
	return url
}

var styleSheetElementCacheMap = typed.SyncMap[*common.Config, template.HTML]{}

func GetStyleSheetLinkElement(config *common.Config) template.HTML {
	if hit, isCached := styleSheetElementCacheMap.Load(config); isCached && !common.KirunaEnv.GetIsDev() {
		return hit
	}

	url := GetStyleSheetURL(config)
	if url == "" {
		styleSheetElementCacheMap.Store(config, "") // Cache the empty string
		return ""
	}

	el := template.HTML(fmt.Sprintf(
		"<link rel=\"stylesheet\" href=\"%s\" id=\"%s\" />", url, StyleSheetElementID,
	))
	styleSheetElementCacheMap.Store(config, el) // Cache the element
	return el
}
