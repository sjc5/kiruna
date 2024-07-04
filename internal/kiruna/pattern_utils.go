package ik

import (
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"
)

type potentialMatch struct {
	pattern string
	path    string
}

func (c *Config) matchResultsKeyMaker(k potentialMatch) string {
	return k.pattern + k.path
}

func (c *Config) getInitialMatchResults(k potentialMatch) (bool, error) {
	normalizedPath := filepath.ToSlash(k.path)

	matches, err := doublestar.Match(k.pattern, normalizedPath)
	if err != nil {
		c.Logger.Errorf("error: failed to match file: %v", err)
		return false, err
	}

	return matches, nil
}

func (c *Config) getIsMatch(k potentialMatch) bool {
	isMatch, _ := c.cache.matchResults.Get(k)
	return isMatch
}

func (c *Config) getIsIgnored(path string, ignoredPatterns *[]string) bool {
	for _, pattern := range *ignoredPatterns {
		if c.getIsMatch(potentialMatch{pattern: pattern, path: path}) {
			return true
		}
	}
	return false
}
