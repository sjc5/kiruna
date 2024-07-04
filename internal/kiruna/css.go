package ik

import (
	"html/template"
	"path/filepath"
	"strings"
)

const (
	CriticalCSSElementID = "__critical-css"
	StyleSheetElementID  = "__normal-css"
)

func (c *Config) getInitialStyleSheetLinkElement() (*template.HTML, error) {
	var result template.HTML

	url := c.GetStyleSheetURL()

	if url != "" {
		var sb strings.Builder
		sb.WriteString(`<link rel="stylesheet" href="`)
		sb.WriteString(url)
		sb.WriteString(`" id="`)
		sb.WriteString(StyleSheetElementID)
		sb.WriteString(`" />`)
		result = template.HTML(sb.String())
	}

	return &result, nil
}

func (c *Config) getInitialStyleSheetURL() (string, error) {
	fs, err := c.GetUniversalFS()
	if err != nil {
		c.Logger.Errorf("error getting FS: %v", err)
		return "", err
	}

	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	content, err := fs.ReadFile(filepath.Join(internalDir, normalCSSFileRefFile))
	if err != nil {
		c.Logger.Errorf("error reading normal CSS URL: %v", err)
		return "", err
	}

	return "/" + filepath.Join(publicDir, string(content)), nil
}

func (c *Config) GetStyleSheetLinkElement() template.HTML {
	res, _ := c.cache.styleSheetLinkElement.Get()
	return *res
}

func (c *Config) GetStyleSheetURL() string {
	url, _ := c.cache.styleSheetURL.Get()
	return url
}

type criticalCSSStatus struct {
	codeStr    string
	noSuchFile bool
	styleEl    template.HTML
}

func (c *Config) getInitialCriticalCSSStatus() (*criticalCSSStatus, error) {
	result := &criticalCSSStatus{}

	// Get FS
	fs, err := c.GetUniversalFS()
	if err != nil {
		c.Logger.Errorf("error getting FS: %v", err)
		return result, err
	}

	// Read critical CSS
	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	content, err := fs.ReadFile(filepath.Join(internalDir, criticalCSSFile))
	if err != nil {
		// Check if the error is a non-existent file, and set the noSuchFile flag in the cache
		result.noSuchFile = strings.HasSuffix(err.Error(), "no such file or directory")

		// if the error was something other than a non-existent file, log it
		if !result.noSuchFile {
			c.Logger.Errorf("error reading critical CSS: %v", err)
		}
		return result, nil
	}

	result.codeStr = string(content)

	// Create style element
	var sb strings.Builder
	sb.WriteString(`<style id="`)
	sb.WriteString(CriticalCSSElementID)
	sb.WriteString(`">`)
	sb.WriteString(result.codeStr)
	sb.WriteString("</style>")

	result.styleEl = template.HTML(sb.String())

	return result, nil
}

func (c *Config) GetCriticalCSS() string {
	result, _ := c.cache.criticalCSS.Get()
	return result.codeStr
}

func (c *Config) GetCriticalCSSStyleElement() template.HTML {
	result, _ := c.cache.criticalCSS.Get()
	return result.styleEl
}

func naiveCSSMinify(content string) string {
	return strings.Join(strings.Fields(content), " ")
}
