package runtime

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/util"
	"github.com/sjc5/kit/pkg/typed"
)

var (
	fileMapFromGlobCacheMap = typed.SyncMap[string, map[string]string]{}
	fileMapLoadOnce         = typed.SyncMap[string, *sync.Once]{}
	urlCacheMap             = typed.SyncMap[string, string]{}
)

func GetPublicURL(config *common.Config, originalPublicURL string, useDirFS bool) string {
	fileMapKey := fmt.Sprintf("%p", config) + fmt.Sprintf("%t", useDirFS)
	urlKey := fileMapKey + originalPublicURL

	if hit, isCached := urlCacheMap.Load(urlKey); isCached {
		return hit
	}

	once, _ := fileMapLoadOnce.LoadOrStore(fileMapKey, &sync.Once{})
	once.Do(func() {
		fileMapFromGob, err := LoadMapFromGob(config, common.PublicFileMapGobName, useDirFS)
		if err != nil {
			util.Log.Errorf("error loading file map from gob: %v", err)
			return
		}
		fileMapFromGlobCacheMap.Store(fileMapKey, fileMapFromGob)
	})

	fileMap, _ := fileMapFromGlobCacheMap.Load(fileMapKey)
	if fileMap == nil {
		return originalPublicURL
	}

	if hashedURL, existsInFileMap := fileMap[cleanURL(originalPublicURL)]; existsInFileMap {
		finalURL := "/public/" + hashedURL
		urlCacheMap.Store(urlKey, finalURL) // Cache the hashed URL
		return finalURL
	}

	// If no hashed URL found, return the original URL
	util.Log.Infof(
		"GetPublicURL: no hashed URL found for %s, returning original URL",
		originalPublicURL,
	)
	finalURL := "/public/" + originalPublicURL
	urlCacheMap.Store(urlKey, finalURL) // Cache the original URL
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
