package ik

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sjc5/kit/pkg/fsutil"
)

const (
	PublicFileMapJSName   = "public_filemap.js"
	PublicFileMapGobName  = "public_filemap.gob"
	PrivateFileMapGobName = "private_filemap.gob"
)

func (c *Config) loadMapFromGob(gobFileName string, useDirFS bool) (map[string]string, error) {
	var FS UniversalFS
	var err error
	if useDirFS {
		FS = c.getUniversalDirFS()
	} else {
		FS, err = c.GetUniversalFS()
	}
	if err != nil {
		return nil, fmt.Errorf("error getting FS: %v", err)
	}

	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	file, err := FS.Open(filepath.Join(internalDir, gobFileName))
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

func (c *Config) saveMapToGob(mapToSave map[string]string, dest string) error {
	file, err := os.Create(filepath.Join(c.getCleanRootDir(), distKirunaDir, internalDir, dest))
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

	hashedFileRefPath := filepath.Join(c.getCleanRootDir(), distKirunaDir, internalDir, publicFileMapFileRefFile)
	if err := os.WriteFile(hashedFileRefPath, []byte(hashedFilename), 0644); err != nil {
		return fmt.Errorf("error writing to file: %v", err)
	}

	return os.WriteFile(filepath.Join(c.getCleanRootDir(), distKirunaDir, staticDir, publicDir, publicInternalDir, hashedFilename), bytes, 0644)
}

func (c *Config) getPreloadPublicFilemapLinkElement() string {
	return fmt.Sprintf(`<link rel="modulepreload" href="%s">`, c.getPublicFileMapURL())
}

func (c *Config) getPublicURLGetterScript() string {
	return fmt.Sprintf(`<script type="module">
	window.kiruna = {}; import { kirunaPublicFileMap } from "%s";
	function getPublicURL(originalPublicURL) { 
		let url = kirunaPublicFileMap[originalPublicURL] || originalPublicURL;
		if (url.startsWith("/")) url = url.slice(1);
		return "/public/" + url;
	}
	window.kiruna.getPublicURL = getPublicURL;
</script>`, c.getPublicFileMapURL())
}

func (c *Config) GetPublicFileMapElements() string {
	return c.getPreloadPublicFilemapLinkElement() + "\n" + c.getPublicURLGetterScript()
}

func (c *Config) getPublicFileMapURL() string {
	if hit, isCached := cache.publicFileMapURL.Load(c); isCached && !KirunaEnv.GetIsDev() {
		return hit
	}

	fs, err := c.GetUniversalFS()
	if err != nil {
		c.Logger.Errorf("error getting FS: %v", err)
		return ""
	}

	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	content, err := fs.ReadFile(filepath.Join(internalDir, publicFileMapFileRefFile))
	if err != nil {
		c.Logger.Errorf("error reading publicFileMapFileRefFile: %v", err)
		return ""
	}

	url := "/" + filepath.Join(publicDir, publicInternalDir, string(content))
	cache.publicFileMapURL.Store(c, url) // Cache the URL
	return url
}
