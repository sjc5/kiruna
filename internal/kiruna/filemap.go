package ik

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sjc5/kit/pkg/fsutil"
	"github.com/sjc5/kit/pkg/htmlutil"
)

const (
	PublicFileMapJSName   = "public_filemap.js"
	PublicFileMapGobName  = "public_filemap.gob"
	PrivateFileMapGobName = "private_filemap.gob"
)

func (c *Config) loadMapFromGob(gobFileName string, isBuildTime bool) (FileMap, error) {
	appropriateFS, err := c.getAppropriateFSMaybeBuildTime(isBuildTime)
	if err != nil {
		return nil, fmt.Errorf("error getting FS: %v", err)
	}

	distKirunaInternal := c.__dist.S().Kiruna.S().Internal

	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	file, err := appropriateFS.Open(filepath.Join(distKirunaInternal.LastSegment(), gobFileName))
	if err != nil {
		return nil, fmt.Errorf("error opening file %s: %v", gobFileName, err)
	}

	defer file.Close()

	var mapFromGob FileMap
	err = fsutil.FromGobInto(file, &mapFromGob)
	if err != nil {
		return nil, fmt.Errorf("error decoding gob: %v", err)
	}
	return mapFromGob, nil
}

func (c *Config) getAppropriateFSMaybeBuildTime(isBuildTime bool) (fs.FS, error) {
	if isBuildTime {
		return c.runtimeCache.baseDirFS.Get()
	}
	return c.GetBaseFS()
}

func (c *Config) saveMapToGob(mapToSave FileMap, dest string) error {
	file, err := os.Create(filepath.Join(c.__dist.S().Kiruna.S().Internal.FullPath(), dest))
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer file.Close()
	encoder := gob.NewEncoder(file)
	return encoder.Encode(mapToSave)
}

func (c *Config) savePublicFileMapJSToInternalPublicDir(mapToSave FileMap) error {
	simpleStrMap := make(map[string]string, len(mapToSave))
	for k, v := range mapToSave {
		simpleStrMap[k] = v.Val
	}

	mapAsJSON, err := json.Marshal(simpleStrMap)
	if err != nil {
		return fmt.Errorf("error marshalling map to JSON: %v", err)
	}

	bytes := []byte(fmt.Sprintf("export const kirunaPublicFileMap = %s;", string(mapAsJSON)))

	hashedFilename := getHashedFilenameFromBytes(bytes, PublicFileMapJSName)

	hashedFileRefPath := c.__dist.S().Kiruna.S().Internal.S().PublicFileMapFileRefDotTXT.FullPath()
	if err := os.WriteFile(hashedFileRefPath, []byte(hashedFilename), 0644); err != nil {
		return fmt.Errorf("error writing to file: %v", err)
	}

	return os.WriteFile(filepath.Join(
		c.__dist.S().Kiruna.S().Static.S().Public.S().PublicInternal.FullPath(),
		hashedFilename,
	), bytes, 0644)
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
	baseFS, err := c.GetBaseFS()
	if err != nil {
		c.Logger.Error(fmt.Sprintf("error getting FS: %v", err))
		return "", err
	}

	distKirunaInternal := c.__dist.S().Kiruna.S().Internal

	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	content, err := fs.ReadFile(baseFS,
		filepath.Join(
			distKirunaInternal.LastSegment(),
			distKirunaInternal.S().PublicFileMapFileRefDotTXT.LastSegment(),
		))
	if err != nil {
		c.Logger.Error(fmt.Sprintf("error reading publicFileMapFileRefFile: %v", err))
		return "", err
	}

	return "/" + filepath.Join(
		PUBLIC,
		c.__dist.S().Kiruna.S().Static.S().Public.S().PublicInternal.LastSegment(),
		string(content),
	), nil
}

func (c *Config) GetPublicFileMapURL() string {
	url, _ := c.runtimeCache.publicFileMapURL.Get()
	return url
}
func (c *Config) GetPublicFileMap() (FileMap, error) {
	return c.runtimeCache.publicFileMapFromGob.Get()
}
func (c *Config) GetPublicFileMapElements() template.HTML {
	details, _ := c.runtimeCache.publicFileMapDetails.Get()
	return details.Elements
}
func (c *Config) GetPublicFileMapScriptSha256Hash() string {
	details, _ := c.runtimeCache.publicFileMapDetails.Get()
	return details.Sha256Hash
}

func (c *Config) GetPublicFileMapKeysBuildtime() ([]string, error) {
	filemap, err := c.getInitialPublicFileMapFromGobBuildtime()
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(filemap))
	for k, v := range filemap {
		if !v.IsPrehashed {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys, nil
}
