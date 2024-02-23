package runtime

import (
	"path/filepath"
	"strings"

	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/util"
)

var fileMapFromGlob common.Map

func GetPublicURL(config *common.Config, originalPublicURL string) string {
	if fileMapFromGlob == nil {
		var err error
		fileMapFromGlob, err = loadMapFromGob(config, common.PublicFileMapGobName)
		if err != nil {
			util.Log.Errorf("error loading file map from gob: %v", err)
			return originalPublicURL
		}
	}
	cleanedOriginalPublicURL := filepath.Clean(originalPublicURL)
	cleanedOriginalPublicURL = strings.TrimPrefix(cleanedOriginalPublicURL, "/")
	hashed, ok := fileMapFromGlob[cleanedOriginalPublicURL]
	if !ok {
		util.Log.Errorf("error getting hashed URL for %s", originalPublicURL)
		return originalPublicURL
	}
	return "/public/hashed/" + hashed
}
