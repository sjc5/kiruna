package ik

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/sjc5/kit/pkg/fsutil"
	"github.com/sjc5/kit/pkg/htmlutil"
)

const (
	PublicFileMapJSName   = "public_filemap.js"
	PublicFileMapGobName  = "public_filemap.gob"
	PrivateFileMapGobName = "private_filemap.gob"
)

func (c *Config) loadMapFromGob(gobFileName string, isBuildTime bool) (map[string]string, error) {
	fs, err := c.getAppropriateFSMaybeBuildTime(isBuildTime)
	if err != nil {
		return nil, fmt.Errorf("error getting FS: %v", err)
	}

	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	file, err := fs.Open(filepath.Join(internalDir, gobFileName))
	if err != nil {
		return nil, fmt.Errorf("error opening file %s: %v", gobFileName, err)
	}

	defer file.Close()

	var mapFromGob map[string]string
	err = fsutil.FromGobInto(file, &mapFromGob)
	if err != nil {
		return nil, fmt.Errorf("error decoding gob: %v", err)
	}
	return mapFromGob, nil
}

func (c *Config) getAppropriateFSMaybeBuildTime(isBuildTime bool) (UniversalFS, error) {
	if isBuildTime {
		return c.cache.uniDirFS.Get()
	}
	return c.GetUniversalFS()
}

func (c *Config) saveMapToGob(mapToSave map[string]string, dest string) error {
	cleanDirs := c.getCleanDirs()

	file, err := os.Create(filepath.Join(cleanDirs.Dist, distKirunaDir, internalDir, dest))
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer file.Close()
	encoder := gob.NewEncoder(file)
	return encoder.Encode(mapToSave)
}

func (c *Config) savePublicFileMapJSToInternalPublicDir(mapToSave map[string]string) error {
	mapAsJSON, err := json.Marshal(mapToSave)
	if err != nil {
		return fmt.Errorf("error marshalling map to JSON: %v", err)
	}

	bytes := []byte(fmt.Sprintf("export const kirunaPublicFileMap = %s;", string(mapAsJSON)))

	hashedFilename := getHashedFilenameFromBytes(bytes, PublicFileMapJSName)

	cleanDirs := c.getCleanDirs()

	hashedFileRefPath := filepath.Join(cleanDirs.Dist, distKirunaDir, internalDir, publicFileMapFileRefFile)
	if err := os.WriteFile(hashedFileRefPath, []byte(hashedFilename), 0644); err != nil {
		return fmt.Errorf("error writing to file: %v", err)
	}

	return os.WriteFile(filepath.Join(cleanDirs.Dist, distKirunaDir, staticDir, publicDir, publicInternalDir, hashedFilename), bytes, 0644)
}

type publicFileMapDetails struct {
	Elements   template.HTML
	Sha256Hash string
}

func (c *Config) getInitialPublicFileMapDetails() (*publicFileMapDetails, error) {
	innerHTMLFormatStr := `
		import { kirunaPublicFileMap } from "%s";
		if (!window.kiruna) window.kiruna = {};
		function getPublicURL(originalPublicURL) { 
			if (originalPublicURL.startsWith("/")) originalPublicURL = originalPublicURL.slice(1);
			return "/public/" + (kirunaPublicFileMap[originalPublicURL] || originalPublicURL);
		}
		window.kiruna.getPublicURL = getPublicURL;` + "\n"

	publicFileMapURL := c.GetPublicFileMapURL()

	linkEl := htmlutil.Element{
		Tag:        "link",
		Attributes: map[string]string{"rel": "modulepreload", "href": publicFileMapURL},
	}

	scriptEl := htmlutil.Element{
		Tag:        "script",
		Attributes: map[string]string{"type": "module"},
		InnerHTML:  template.HTML(fmt.Sprintf(innerHTMLFormatStr, publicFileMapURL)),
	}

	sha256Hash, err := htmlutil.AddSha256HashInline(&scriptEl, true)
	if err != nil {
		return nil, fmt.Errorf("error handling CSP: %v", err)
	}

	var htmlBuilder strings.Builder

	err = htmlutil.RenderElementToBuilder(&linkEl, &htmlBuilder)
	if err != nil {
		return nil, fmt.Errorf("error rendering element to builder: %v", err)
	}
	err = htmlutil.RenderElementToBuilder(&scriptEl, &htmlBuilder)
	if err != nil {
		return nil, fmt.Errorf("error rendering element to builder: %v", err)
	}

	return &publicFileMapDetails{
		Elements:   template.HTML(htmlBuilder.String()),
		Sha256Hash: sha256Hash,
	}, nil
}

func (c *Config) getInitialPublicFileMapURL() (string, error) {
	fs, err := c.GetUniversalFS()
	if err != nil {
		c.Logger.Error(fmt.Sprintf("error getting FS: %v", err))
		return "", err
	}

	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	content, err := fs.ReadFile(filepath.Join(internalDir, publicFileMapFileRefFile))
	if err != nil {
		c.Logger.Error(fmt.Sprintf("error reading publicFileMapFileRefFile: %v", err))
		return "", err
	}

	return "/" + filepath.Join(publicDir, publicInternalDir, string(content)), nil
}

func (c *Config) GetPublicFileMapURL() string {
	url, _ := c.cache.publicFileMapURL.Get()
	return url
}
func (c *Config) GetPublicFileMap() (map[string]string, error) {
	return c.cache.publicFileMapFromGob.Get()
}
func (c *Config) GetPublicFileMapElements() template.HTML {
	details, _ := c.cache.publicFileMapDetails.Get()
	return details.Elements
}
func (c *Config) GetPublicFileMapScriptSha256Hash() string {
	details, _ := c.cache.publicFileMapDetails.Get()
	return details.Sha256Hash
}

func (c *Config) GetPublicFileMapKeysBuildtime(excludedPrefixes []string) ([]string, error) {
	filemap, err := c.getInitialPublicFileMapFromGobBuildtime()
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(filemap))
	for k := range filemap {
		shouldAppend := true
		for _, prefix := range excludedPrefixes {
			if strings.HasPrefix(k, prefix) {
				shouldAppend = false
				break
			}
		}
		if shouldAppend {
			keys = append(keys, k)
		}
	}
	return keys, nil
}
