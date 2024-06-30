package ik

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
)

func (c *Config) GetServeStaticHandler(pathPrefix string, cacheImmutably bool) http.Handler {
	FS, err := c.GetPublicFS()
	if err != nil {
		c.Logger.Errorf("error getting public FS: %v", err)
	}
	if cacheImmutably {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			http.StripPrefix(pathPrefix, http.FileServer(http.FS(FS))).ServeHTTP(w, r)
		})
	}
	return http.StripPrefix(pathPrefix, http.FileServer(http.FS(FS)))
}

func (c *Config) GetPublicURL(originalPublicURL string, useDirFS bool) string {
	fileMapKey := fmt.Sprintf("%p", c) + fmt.Sprintf("%t", useDirFS)
	urlKey := fileMapKey + originalPublicURL

	if hit, isCached := cache.publicURLs.Load(urlKey); isCached {
		return hit
	}

	once, _ := cache.fileMapLoadOnce.LoadOrStore(fileMapKey, &sync.Once{})
	var fileMapFromGob map[string]string
	once.Do(func() {
		var err error
		fileMapFromGob, err = c.LoadMapFromGob(PublicFileMapGobName, useDirFS)
		if err != nil {
			c.Logger.Errorf("error loading file map from gob: %v", err)
			return
		}
		cache.fileMapFromGob.Store(fileMapKey, fileMapFromGob)
	})

	if fileMapFromGob == nil {
		fileMapFromGob, _ = cache.fileMapFromGob.Load(fileMapKey)
	}
	if fileMapFromGob == nil {
		return originalPublicURL
	}

	if hashedURL, existsInFileMap := fileMapFromGob[cleanURL(originalPublicURL)]; existsInFileMap {
		finalURL := "/" + publicDir + "/" + hashedURL
		cache.publicURLs.Store(urlKey, finalURL) // Cache the hashed URL
		return finalURL
	}

	// If no hashed URL found, return the original URL
	c.Logger.Infof(
		"GetPublicURL: no hashed URL found for %s, returning original URL",
		originalPublicURL,
	)
	finalURL := "/" + publicDir + "/" + originalPublicURL
	cache.publicURLs.Store(urlKey, finalURL) // Cache the original URL
	return finalURL
}

func (c *Config) MakePublicURLsMap(filepaths []string, useDirFS bool) map[string]string {
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
		filepathsMap[safeKey] = c.GetPublicURL(filepath, useDirFS)
	}

	return filepathsMap
}

func cleanURL(url string) string {
	return strings.TrimPrefix(filepath.Clean(url), "/")
}
