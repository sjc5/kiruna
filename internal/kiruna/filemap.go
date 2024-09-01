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

func (c *Config) loadMapFromGob(gobFileName string) (map[string]string, error) {
	var FS UniversalFS
	var err error
	if getIsBuildTime() {
		FS, err = c.cache.uniDirFS.Get()
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

func (c *Config) GetPublicFileMapElements() string {
	formatStr := `
		<link rel="modulepreload" href="%s">
		<script type="module">
			import { kirunaPublicFileMap } from "%s";
			if (!window.kiruna) window.kiruna = {};
			function getPublicURL(originalPublicURL) { 
				if (originalPublicURL.startsWith("/")) originalPublicURL = originalPublicURL.slice(1);
				return "/public/" + (kirunaPublicFileMap[originalPublicURL] || originalPublicURL);
			}
			window.kiruna.getPublicURL = getPublicURL;
		</script>
		`

	return fmt.Sprintf(formatStr, c.GetPublicFileMapURL(), c.GetPublicFileMapURL())
}

func (c *Config) getInitialPublicFileMapURL() (string, error) {
	fs, err := c.GetUniversalFS()
	if err != nil {
		c.Logger.Errorf("error getting FS: %v", err)
		return "", err
	}

	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	content, err := fs.ReadFile(filepath.Join(internalDir, publicFileMapFileRefFile))
	if err != nil {
		c.Logger.Errorf("error reading publicFileMapFileRefFile: %v", err)
		return "", err
	}

	return "/" + filepath.Join(publicDir, publicInternalDir, string(content)), nil
}

func (c *Config) GetPublicFileMapURL() string {
	url, _ := c.cache.publicFileMapURL.Get()
	return url
}
