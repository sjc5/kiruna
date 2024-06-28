package runtime

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/util"
)

var fileMapFromGlobCacheMap = make(map[string]*map[string]string)
var urlCacheMap = make(map[string]string)

func GetPublicURL(config *common.Config, originalPublicURL string, useDirFS bool) string {
	fileMapKey := fmt.Sprintf("%p", config) + fmt.Sprintf("%t", useDirFS)
	urlKey := fileMapKey + originalPublicURL

	if hit, isCached := urlCacheMap[urlKey]; isCached {
		return hit
	}

	if fileMapFromGlobCacheMap[fileMapKey] == nil {
		fileMapFromGob, err := LoadMapFromGob(config, common.PublicFileMapGobName, useDirFS)
		if err != nil {
			util.Log.Errorf("error loading file map from gob: %v", err)
			return originalPublicURL
		}
		fileMapFromGlobCacheMap[fileMapKey] = &fileMapFromGob
	}

	fileMap := *fileMapFromGlobCacheMap[fileMapKey]

	if hashedURL, existsInFileMap := fileMap[cleanURL(originalPublicURL)]; existsInFileMap {
		finalURL := "/public/" + hashedURL
		urlCacheMap[urlKey] = finalURL // Cache the hashed URL
		return finalURL
	}

	// If no hashed URL found, return the original URL
	util.Log.Infof(
		"GetPublicURL: no hashed URL found for %s, returning original URL",
		originalPublicURL,
	)
	finalURL := "/public/" + originalPublicURL
	urlCacheMap[urlKey] = finalURL // Cache the original URL
	return finalURL
}

func cleanURL(url string) string {
	return strings.TrimPrefix(filepath.Clean(url), "/")
}

func MakePublicURLsMap(config *common.Config, filepaths []string, useDirFS bool) map[string]string {
	filepathsMap := make(map[string]string, len(filepaths))
	var sb strings.Builder
	sb.Grow(64)

	for _, filepath := range filepaths {
		sb.Reset()
		for _, r := range filepath {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				sb.WriteRune(r)
			} else {
				sb.WriteRune('_')
			}
		}
		safeKey := sb.String()
		filepathsMap[safeKey] = GetPublicURL(config, filepath, useDirFS)
	}

	return filepathsMap
}
