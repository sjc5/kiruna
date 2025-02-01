package ik

import (
	"fmt"
	"html/template"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/sjc5/kit/pkg/htmlutil"
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
	baseFS, err := c.GetBaseFS()
	if err != nil {
		c.Logger.Error(fmt.Sprintf("error getting FS: %v", err))
		return "", err
	}

	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	content, err := fs.ReadFile(baseFS, filepath.Join(internalDir, normalCSSFileRefFile))
	if err != nil {
		c.Logger.Error(fmt.Sprintf("error reading normal CSS URL: %v", err))
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
	sha256Hash string
}

func (c *Config) getInitialCriticalCSSStatus() (*criticalCSSStatus, error) {
	result := &criticalCSSStatus{}

	// Get FS
	baseFS, err := c.GetBaseFS()
	if err != nil {
		c.Logger.Error(fmt.Sprintf("error getting FS: %v", err))
		return nil, err
	}

	// Read critical CSS
	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	content, err := fs.ReadFile(baseFS, filepath.Join(internalDir, criticalCSSFile))
	if err != nil {
		// Check if the error is a non-existent file, and set the noSuchFile flag in the cache
		result.noSuchFile = strings.HasSuffix(err.Error(), "no such file or directory")

		if !result.noSuchFile {
			c.Logger.Error(fmt.Sprintf("error reading critical CSS: %v", err))
			return nil, err
		}
		return result, nil
	}

	result.codeStr = string(content)

	el := htmlutil.Element{
		Tag:               "style",
		TrustedAttributes: map[string]string{"id": CriticalCSSElementID},
		InnerHTML:         template.HTML(result.codeStr),
	}

	sha256Hash, err := htmlutil.AddSha256HashInline(&el, true)
	if err != nil {
		c.Logger.Error(fmt.Sprintf("error handling CSP: %v", err))
		return nil, err
	}
	result.sha256Hash = sha256Hash

	tpmlRes, err := htmlutil.RenderElement(&el)
	if err != nil {
		c.Logger.Error(fmt.Sprintf("error rendering element: %v", err))
		return nil, err
	}

	result.styleEl = tpmlRes

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
func (c *Config) GetCriticalCSSStyleElementSha256Hash() string {
	result, _ := c.cache.criticalCSS.Get()
	return result.sha256Hash
}
