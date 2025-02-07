package ik

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/sjc5/kit/pkg/errutil"
	"github.com/sjc5/kit/pkg/fsutil"
	"github.com/sjc5/kit/pkg/typed"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

var noHashPublicDirsByVersion = map[uint8]string{0: "__nohash", 1: "prehashed"}

func (c *Config) Build(recompileBinary bool, shouldBeGranular bool) error {
	enforceProperInstantiation(c)

	c.fileSemaphore = semaphore.NewWeighted(100)

	if !shouldBeGranular {

		// nuke the dist/kiruna directory
		if err := os.RemoveAll(c.__dist.S().Kiruna.FullPath()); err != nil {
			return fmt.Errorf("error removing dist/kiruna directory: %v", err)
		}

		// re-make required directories
		if err := c.SetupDistDir(); err != nil {
			return fmt.Errorf("error making requisite directories: %v", err)
		}
	}

	if !c.ServerOnly {
		// Must be complete before BuildCSS in case the CSS references any public files
		if err := c.handlePublicFiles(shouldBeGranular); err != nil {
			return fmt.Errorf("error handling public files: %v", err)
		}

		var eg errgroup.Group
		eg.Go(func() error {
			return errutil.Maybe("error during precompile task (copyPrivateFiles)", c.copyPrivateFiles(shouldBeGranular))
		})
		eg.Go(func() error {
			return errutil.Maybe("error during precompile task (buildCSS)", c.buildCSS())
		})
		if err := eg.Wait(); err != nil {
			return err
		}
	}

	if recompileBinary {
		if err := c.compileBinary(); err != nil {
			return fmt.Errorf("error compiling binary: %v", err)
		}
	}
	return nil
}

func (c *Config) buildCSS() error {
	err := c.processCSS("critical")
	if err != nil {
		return fmt.Errorf("error processing critical CSS: %v", err)
	}

	err = c.processCSS("normal")
	if err != nil {
		return fmt.Errorf("error processing normal CSS: %v", err)
	}

	return nil
}

// ProcessCSS concatenates and hashes specified CSS files, then saves them to disk.
func (c *Config) processCSS(subDir string) error {
	dirPath := filepath.Join(c.cleanSrcDirs.Styles, subDir)
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return nil
	}
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("error reading directory: %v", err)
	}

	var fileNames []string

	// Collect and sort .css files
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".css") {
			fileNames = append(fileNames, file.Name())
		}
	}
	sort.Strings(fileNames)

	var wg sync.WaitGroup

	processedCSS := make([]string, len(fileNames))

	for i, fileName := range fileNames {
		wg.Add(1)
		go func(fn string) {
			defer wg.Done()
			if err := c.fileSemaphore.Acquire(context.Background(), 1); err != nil {
				c.Logger.Error(fmt.Sprintf("error acquiring semaphore: %v", err))
				return
			}
			defer c.fileSemaphore.Release(1)

			content, err := os.ReadFile(filepath.Join(dirPath, fn))
			if err != nil {
				c.Logger.Error("error reading file %s: %v", fn, err)
				return
			}
			processedCSS[i] = string(content)
		}(fileName)
	}

	wg.Wait()

	var concatenatedCSS strings.Builder
	for _, css := range processedCSS {
		concatenatedCSS.WriteString(css)
	}

	concatenatedCSSString := concatenatedCSS.String()
	concatenatedCSSString = c.ResolveCSSURLFuncArgs(concatenatedCSSString)

	// Determine output path and filename
	var outputPath string

	switch subDir {
	case "critical":
		outputPath = c.__dist.S().Kiruna.S().Internal.FullPath()
	case "normal":
		outputPath = c.__dist.S().Kiruna.S().Static.S().Public.FullPath()
	}

	outputFileName := subDir + ".css" // Default for 'critical'

	if subDir == "normal" {
		// first, delete the old normal.css file(s)
		oldNormalPath := filepath.Join(outputPath, "normal_*.css")
		oldNormalFiles, err := filepath.Glob(oldNormalPath)
		if err != nil {
			return fmt.Errorf("error finding old normal CSS files: %v", err)
		}
		for _, oldNormalFile := range oldNormalFiles {
			if err := os.Remove(oldNormalFile); err != nil {
				return fmt.Errorf("error removing old normal CSS file: %v", err)
			}
		}

		// Hash the concatenated content
		outputFileName = getHashedFilenameFromBytes([]byte(concatenatedCSSString), "normal.css")
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return fmt.Errorf("error creating output directory: %v", err)
	}

	// Write concatenated content to file
	outputFile := filepath.Join(outputPath, outputFileName)

	// If normal, also write to a file called normal_css_ref.txt with the hash
	if subDir == "normal" {
		hashFile := c.__dist.S().Kiruna.S().Internal.S().NormalCSSFileRefDotTXT.FullPath()
		if err := os.WriteFile(hashFile, []byte(outputFileName), 0644); err != nil {
			return fmt.Errorf("error writing to file: %v", err)
		}
	}

	finalCSS := concatenatedCSSString

	if !GetIsDev() {
		m := minify.New()
		m.AddFunc("text/css", css.Minify)
		finalCSS, err = m.String("text/css", concatenatedCSSString)
		if err != nil {
			return fmt.Errorf("error minifying CSS: %v", err)
		}
	}

	return os.WriteFile(outputFile, []byte(finalCSS), 0644)
}

type staticFileProcessorOpts struct {
	basename         string
	srcDir           string
	distDir          string
	mapName          string
	shouldBeGranular bool
	getIsNoHashDir   func(string) (bool, uint8)
	writeWithHash    bool
}

func (c *Config) handlePublicFiles(shouldBeGranular bool) error {
	return c.processStaticFiles(&staticFileProcessorOpts{
		basename:         PUBLIC,
		srcDir:           c.cleanSrcDirs.PublicStatic,
		distDir:          c.__dist.S().Kiruna.S().Static.S().Public.FullPath(),
		mapName:          PublicFileMapGobName,
		shouldBeGranular: shouldBeGranular,
		getIsNoHashDir: func(path string) (bool, uint8) {
			if strings.HasPrefix(path, noHashPublicDirsByVersion[1]) {
				return true, 1
			}
			if strings.HasPrefix(path, noHashPublicDirsByVersion[0]) {
				return true, 0
			}
			return false, 0
		},
		writeWithHash: true,
	})
}

func (c *Config) copyPrivateFiles(shouldBeGranular bool) error {
	return c.processStaticFiles(&staticFileProcessorOpts{
		basename:         PRIVATE,
		srcDir:           c.cleanSrcDirs.PrivateStatic,
		distDir:          c.__dist.S().Kiruna.S().Static.S().Private.FullPath(),
		mapName:          PrivateFileMapGobName,
		shouldBeGranular: shouldBeGranular,
		getIsNoHashDir: func(path string) (bool, uint8) {
			return false, 0
		},
		writeWithHash: false,
	})
}

type fileInfo struct {
	path         string
	relativePath string
	isNoHashDir  bool
}

func (c *Config) processStaticFiles(opts *staticFileProcessorOpts) error {
	if _, err := os.Stat(opts.srcDir); os.IsNotExist(err) {
		return nil
	}

	newFileMap := typed.SyncMap[string, string]{}
	oldFileMap := typed.SyncMap[string, string]{}

	// Load old file map if granular updates are enabled
	if opts.shouldBeGranular {
		var err error
		oldMap, err := c.loadMapFromGob(opts.mapName, true)
		if err != nil {
			return fmt.Errorf("error reading old file map: %v", err)
		}
		for k, v := range oldMap {
			oldFileMap.Store(k, v)
		}
	}

	fileChan := make(chan fileInfo, 100)
	errChan := make(chan error, 1)
	var wg sync.WaitGroup

	// File discovery goroutine
	go func() {
		defer close(fileChan)
		err := filepath.WalkDir(opts.srcDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				relativePath, err := filepath.Rel(opts.srcDir, path)
				if err != nil {
					return err
				}
				relativePath = filepath.ToSlash(relativePath)
				isNoHashDir, version := opts.getIsNoHashDir(relativePath)
				if isNoHashDir {
					relativePath = strings.TrimPrefix(relativePath, noHashPublicDirsByVersion[version]+"/")
				}
				fileChan <- fileInfo{path: path, relativePath: relativePath, isNoHashDir: isNoHashDir}
			}
			return nil
		})
		if err != nil {
			errChan <- err
		}
	}()

	// File processing goroutines
	workerCount := 4
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fi := range fileChan {
				if err := c.processFile(fi, opts, &newFileMap, &oldFileMap, opts.distDir); err != nil {
					errChan <- err
					return
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	if err := <-errChan; err != nil {
		return err
	}

	// Cleanup old moot files if granular updates are enabled
	if opts.shouldBeGranular {
		var oldMapErr error
		oldFileMap.Range(func(k, v string) bool {
			if newHash, exists := newFileMap.Load(k); !exists || newHash != v {
				oldDistPath := filepath.Join(opts.distDir, v)
				err := os.Remove(oldDistPath)
				if err != nil && !os.IsNotExist(err) {
					oldMapErr = fmt.Errorf(
						"error removing old static file from dist (%s/%s): %v", opts.basename, v, err,
					)
					return false
				}
			}
			return true
		})
		if oldMapErr != nil {
			return oldMapErr
		}
	}

	// Save the updated file map
	err := c.saveMapToGob(toStdMap(&newFileMap), opts.mapName)
	if err != nil {
		return fmt.Errorf("error saving file map: %v", err)
	}

	if opts.basename == PUBLIC {
		err = c.savePublicFileMapJSToInternalPublicDir(toStdMap(&newFileMap))
		if err != nil {
			return fmt.Errorf("error saving public file map JSON: %v", err)
		}
	}

	return nil
}

func (c *Config) processFile(fi fileInfo, opts *staticFileProcessorOpts, newFileMap, oldFileMap *typed.SyncMap[string, string], distDir string) error {
	if err := c.fileSemaphore.Acquire(context.Background(), 1); err != nil {
		return fmt.Errorf("error acquiring semaphore: %v", err)
	}
	defer c.fileSemaphore.Release(1)

	relativePathUnderscores := strings.ReplaceAll(fi.relativePath, "/", "_")

	var fileIdentifier string
	if fi.isNoHashDir {
		fileIdentifier = fi.relativePath
	} else {
		var err error
		fileIdentifier, err = getHashedFilenameFromPath(fi.path, relativePathUnderscores)
		if err != nil {
			return fmt.Errorf("error getting hashed filename: %v", err)
		}
	}

	newFileMap.Store(fi.relativePath, fileIdentifier)

	// Skip unchanged files if granular updates are enabled
	if opts.shouldBeGranular {
		if oldHash, exists := oldFileMap.Load(fi.relativePath); exists && oldHash == fileIdentifier {
			return nil
		}
	}

	var distPath string
	if opts.writeWithHash {
		distPath = filepath.Join(distDir, fileIdentifier)
	} else {
		distPath = filepath.Join(distDir, fi.relativePath)
	}

	err := os.MkdirAll(filepath.Dir(distPath), 0755)
	if err != nil {
		return fmt.Errorf("error creating directory: %v", err)
	}

	err = fsutil.CopyFile(fi.path, distPath)
	if err != nil {
		return fmt.Errorf("error copying file: %v", err)
	}

	return nil
}

var urlRegex = regexp.MustCompile(`url\(([^)]+)\)`)

func (c *Config) ResolveCSSURLFuncArgs(css string) string {
	return urlRegex.ReplaceAllStringFunc(css, func(match string) string {
		rawUrl := urlRegex.FindStringSubmatch(match)[1]
		cleanedUrl := strings.TrimSpace(strings.Trim(rawUrl, "'\""))
		if !strings.HasPrefix(cleanedUrl, "http") && !strings.Contains(cleanedUrl, "://") {
			hashedUrl := c.MustGetPublicURLBuildtime(cleanedUrl)
			return fmt.Sprintf("url(%s)", hashedUrl)
		} else {
			return match // Leave external URLs unchanged
		}
	})
}

func toStdMap(sm *typed.SyncMap[string, string]) map[string]string {
	m := map[string]string{}
	sm.Range(func(k, v string) bool {
		m[k] = v
		return true
	})
	return m
}
