package ik

import (
	"net/http"
	"path/filepath"
	"strings"
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
	if strings.HasPrefix(originalPublicURL, "data:") {
		return originalPublicURL, nil
	}

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

func publicURLsKeyMaker(x string) string { return x }

func (c *Config) GetPublicURL(originalPublicURL string) string {
	url, _ := c.cache.publicURLs.Get(originalPublicURL)
	return url
}

func cleanURL(url string) string {
	return strings.TrimPrefix(filepath.Clean(url), "/")
}
