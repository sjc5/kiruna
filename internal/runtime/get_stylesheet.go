package runtime

import (
	"fmt"
	"html/template"
	"path/filepath"

	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/util"
)

const StyleSheetElementID = "__normal-css"

func GetStyleSheetURL(config *common.Config) string {
	FS, err := GetUniversalFS(config)
	if err != nil {
		util.Log.Errorf("error getting FS: %v", err)
		return ""
	}
	content, err := FS.ReadFile(filepath.Join("kiruna", "internal", "normal_css_file_ref.txt"))
	if err != nil {
		util.Log.Errorf("error reading normal CSS URL: %v", err)
		return ""
	}
	return "/public/" + string(content)
}

func GetStyleSheetLinkElement(config *common.Config) template.HTML {
	url := GetStyleSheetURL(config)
	if url == "" {
		return ""
	}
	return template.HTML(fmt.Sprintf("<link rel=\"stylesheet\" href=\"%s\" id=\"%s\" />", url, StyleSheetElementID))
}
