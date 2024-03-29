package runtime

import (
	"path/filepath"
	"strings"

	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/util"
)

var fileMapFromGlob common.Map

func GetPublicURL(config *common.Config, originalPublicURL string, useDirFS bool) string {
	if fileMapFromGlob == nil {
		var err error
		fileMapFromGlob, err = loadMapFromGob(config, common.PublicFileMapGobName, useDirFS)
		if err != nil {
			util.Log.Errorf("error loading file map from gob: %v", err)
			return originalPublicURL
		}
	}
	cleanedOriginalPublicURL := filepath.Clean(originalPublicURL)
	cleanedOriginalPublicURL = strings.TrimPrefix(cleanedOriginalPublicURL, "/")
	hashed, ok := fileMapFromGlob[cleanedOriginalPublicURL]
	if !ok {
		util.Log.Infof("GetPublicURL: no hashed URL found for %s, returning original URL", originalPublicURL)
		return "/public/" + originalPublicURL
	}
	return "/public/" + hashed
}
