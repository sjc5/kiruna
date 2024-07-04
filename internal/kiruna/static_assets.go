package ik

import (
	"net/http"
	"path/filepath"
	"strings"
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

func (c *Config) getInitialPublicFileMapFromGob() (map[string]string, error) {
	return c.loadMapFromGob(PublicFileMapGobName)
}

func (c *Config) getInitialPublicURL(originalPublicURL string) (string, error) {
	fileMapFromGob, err := c.cache.publicFileMapFromGob.Get()
	if err != nil {
		c.Logger.Errorf("error getting public file map from gob: %v", err)
		return originalPublicURL, err
	}

	if hashedURL, existsInFileMap := fileMapFromGob[cleanURL(originalPublicURL)]; existsInFileMap {
		return "/" + publicDir + "/" + hashedURL, nil
	}

	// If no hashed URL found, return the original URL
	c.Logger.Infof(
		"GetPublicURL: no hashed URL found for %s, returning original URL",
		originalPublicURL,
	)

	return "/" + publicDir + "/" + originalPublicURL, nil
}

func (c *Config) GetPublicURL(originalPublicURL string) string {
	url, _ := c.cache.publicURLs.Get(originalPublicURL)
	return url
}

func (c *Config) getInitialPublicURLsMap(filepaths []string) (map[string]string, error) {
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
		filepathsMap[sb.String()] = c.GetPublicURL(filepath)
	}

	return filepathsMap, nil
}

func (c *Config) publicFileMapKeyMaker(filepaths []string) string {
	return c.getPublicFileMapURL() + strings.Join(filepaths, "")
}

func (c *Config) MakePublicURLsMap(filepaths []string) map[string]string {
	urlsMap, _ := c.cache.publicURLsMap.Get(filepaths)
	return urlsMap
}

func cleanURL(url string) string {
	return strings.TrimPrefix(filepath.Clean(url), "/")
}
