package ik

import (
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"
)

func (c *Config) getIsMatch(pattern string, path string) bool {
	combined := pattern + path

	if hit, isCached := cache.matchResults.Load(combined); isCached {
		return hit
	}

	normalizedPath := filepath.ToSlash(path)

	matches, err := doublestar.Match(pattern, normalizedPath)
	if err != nil {
		c.Logger.Errorf("error: failed to match file: %v", err)
		return false
	}

	actualValue, _ := cache.matchResults.LoadOrStore(combined, matches)
	return actualValue
}

func (c *Config) getIsIgnored(path string, ignoredPatterns *[]string) bool {
	for _, pattern := range *ignoredPatterns {
		if c.getIsMatch(pattern, path) {
			return true
		}
	}
	return false
}
