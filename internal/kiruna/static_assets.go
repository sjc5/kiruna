package ik

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
)

func (c *Config) GetServeStaticHandler(pathPrefix string, addImmutableCacheHeaders bool) (http.Handler, error) {
	publicFS, err := c.GetPublicFS()
	if err != nil {
		errMsg := fmt.Sprintf("error getting public FS: %v", err)
		c.Logger.Error(errMsg)
		return nil, errors.New(errMsg)
	}
	if addImmutableCacheHeaders {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			http.StripPrefix(pathPrefix, http.FileServer(http.FS(publicFS))).ServeHTTP(w, r)
		}), nil
	}
	return http.StripPrefix(pathPrefix, http.FileServer(http.FS(publicFS))), nil
}

func (c *Config) getInitialPublicFileMapFromGobBuildtime() (map[string]string, error) {
	return c.loadMapFromGob(PublicFileMapGobName, true)
}

func (c *Config) getInitialPublicFileMapFromGobRuntime() (map[string]string, error) {
	return c.loadMapFromGob(PublicFileMapGobName, false)
}

func (c *Config) getPublicURLBuildtime(originalPublicURL string) (string, error) {
	fileMapFromGob, err := c.getInitialPublicFileMapFromGobBuildtime()
	if err != nil {
		c.Logger.Error(fmt.Sprintf(
			"error getting public file map from gob for originalPublicURL %s: %v", originalPublicURL, err,
		))
		return "/" + publicDir + "/" + originalPublicURL, err
	}

	return c.getInitialPublicURLInner(originalPublicURL, fileMapFromGob)
}

func (c *Config) getInitialPublicURL(originalPublicURL string) (string, error) {
	fileMapFromGob, err := c.runtimeCache.publicFileMapFromGob.Get()
	if err != nil {
		c.Logger.Error(fmt.Sprintf(
			"error getting public file map from gob for originalPublicURL %s: %v", originalPublicURL, err,
		))
		return "/" + publicDir + "/" + originalPublicURL, err
	}

	return c.getInitialPublicURLInner(originalPublicURL, fileMapFromGob)
}

func (c *Config) getInitialPublicURLInner(originalPublicURL string, fileMapFromGob map[string]string) (string, error) {
	if strings.HasPrefix(originalPublicURL, "data:") {
		return originalPublicURL, nil
	}

	if hashedURL, existsInFileMap := fileMapFromGob[cleanURL(originalPublicURL)]; existsInFileMap {
		return "/" + publicDir + "/" + hashedURL, nil
	}

	// If no hashed URL found, return the original URL
	c.Logger.Info(fmt.Sprintf(
		"GetPublicURL: no hashed URL found for %s, returning original URL",
		originalPublicURL,
	))

	return "/" + publicDir + "/" + originalPublicURL, nil
}

func publicURLsKeyMaker(x string) string { return x }

func (c *Config) GetPublicURL(originalPublicURL string) string {
	url, _ := c.runtimeCache.publicURLs.Get(originalPublicURL)
	return url
}

func cleanURL(url string) string {
	return strings.TrimPrefix(filepath.Clean(url), "/")
}
