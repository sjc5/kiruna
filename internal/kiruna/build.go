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

	"github.com/sjc5/kit/pkg/fsutil"
	"golang.org/x/sync/semaphore"
)

var fileSemaphore = semaphore.NewWeighted(100)

type syncMap struct {
	sync.RWMutex
	m map[string]string
}

func (sm *syncMap) Store(key, value string) {
	sm.Lock()
	defer sm.Unlock()
	sm.m[key] = value
}

func (sm *syncMap) Load(key string) (string, bool) {
	sm.RLock()
	defer sm.RUnlock()
	v, ok := sm.m[key]
	return v, ok
}

type precompileError struct {
	task string
	err  error
}

func (e precompileError) Error() string {
	return fmt.Sprintf("error during precompile task %s: %v", e.task, e.err)
}

func (c *Config) Build(recompileBinary bool, shouldBeGranular bool) error {
	cleanRootDir := c.getCleanRootDir()

	if !shouldBeGranular {

		// check for existing PID file
		pidFile := PIDFile{cleanRootDir: cleanRootDir}
		lastPID, err := pidFile.readPIDFile()
		if err != nil {
			return fmt.Errorf("error reading PID file: %v", err)
		}

		// nuke the dist/kiruna directory
		if err := os.RemoveAll(filepath.Join(cleanRootDir, distKirunaDir)); err != nil {
			return fmt.Errorf("error removing dist/kiruna directory: %v", err)
		}

		// re-make required directories
		isServerOnly := c.DevConfig != nil && c.DevConfig.ServerOnly
		if !isServerOnly {
			if err := SetupDistDir(c.RootDir); err != nil {
				return fmt.Errorf("error making requisite directories: %v", err)
			}
		}

		// add pid file back
		if lastPID != 0 {
			if err := pidFile.writePIDFile(lastPID); err != nil {
				return fmt.Errorf("error writing PID file: %v", err)
			}
		}
	}

	// Must be complete before BuildCSS in case the CSS references any public files
	if err := c.handlePublicFiles(shouldBeGranular); err != nil {
		return fmt.Errorf("error handling public files: %v", err)
	}

	// Concurrently execute tasks
	var wg sync.WaitGroup
	errChan := make(chan error, 2) // Buffer to hold up to 2 errors
	wg.Add(2)                      // Two tasks to do concurrently

	// goroutine 1
	go func() {
		defer wg.Done()
		if err := c.copyPrivateFiles(shouldBeGranular); err != nil {
			errChan <- precompileError{task: "copyPrivateFiles", err: err}
		}
	}()

	// goroutine 2
	go func() {
		defer wg.Done()
		if err := c.buildCSS(); err != nil {
			errChan <- precompileError{task: "BuildCSS", err: err}
		}
	}()

	// Wait for all tasks to complete
	wg.Wait()
	close(errChan)

	var combinedErrors []error
	for e := range errChan {
		if e != nil {
			combinedErrors = append(combinedErrors, e)
		}
	}

	if len(combinedErrors) > 0 {
		return fmt.Errorf("multiple errors: %v", combinedErrors)
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

var urlRegex = regexp.MustCompile(`url\(([^)]+)\)`)

type syncString struct {
	sync.RWMutex
	builder strings.Builder
}

func (s *syncString) append(str string) {
	s.Lock()
	defer s.Unlock()
	s.builder.WriteString(str)
}

func (s *syncString) string() string {
	s.RLock()
	defer s.RUnlock()
	return s.builder.String()
}

// ProcessCSS concatenates and hashes specified CSS files, then saves them to disk.
func (c *Config) processCSS(subDir string) error {
	cleanRootDir := c.getCleanRootDir()

	dirPath := filepath.Join(cleanRootDir, "styles", subDir)
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

	var concatenatedCSS syncString
	var wg sync.WaitGroup

	for _, fileName := range fileNames {
		wg.Add(1)
		go func(fn string) {
			defer wg.Done()
			if err := fileSemaphore.Acquire(context.Background(), 1); err != nil {
				c.Logger.Errorf("Error acquiring semaphore: %v", err)
				return
			}
			defer fileSemaphore.Release(1)

			content, err := os.ReadFile(filepath.Join(dirPath, fn))
			if err != nil {
				c.Logger.Errorf("Error reading file %s: %v", fn, err)
				return
			}
			concatenatedCSS.append(string(content))
		}(fileName)
	}

	wg.Wait()

	concatenatedCSSString := concatenatedCSS.string()
	concatenatedCSSString = urlRegex.ReplaceAllStringFunc(concatenatedCSSString, func(match string) string {
		rawUrl := urlRegex.FindStringSubmatch(match)[1]
		cleanedUrl := strings.TrimSpace(strings.Trim(rawUrl, "'\""))
		if !strings.HasPrefix(cleanedUrl, "http") && !strings.Contains(cleanedUrl, "://") {
			hashedUrl := c.GetPublicURL(cleanedUrl, true)
			return fmt.Sprintf("url(%s)", hashedUrl)
		} else {
			return match // Leave external URLs unchanged
		}
	})

	// Determine output path and filename
	var outputPath string

	switch subDir {
	case "critical":
		outputPath = filepath.Join(cleanRootDir, distKirunaDir, internalDir)
	case "normal":
		outputPath = filepath.Join(cleanRootDir, distKirunaDir, staticDir, publicDir)
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
		outputFileName = getHashedFilenameFromBytes([]byte(concatenatedCSS.string()), "normal.css")
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return fmt.Errorf("error creating output directory: %v", err)
	}

	// Write concatenated content to file
	outputFile := filepath.Join(outputPath, outputFileName)

	// If normal, also write to a file called normal_css_ref.txt with the hash
	if subDir == "normal" {
		hashFile := filepath.Join(cleanRootDir, distKirunaDir, internalDir, normalCSSFileRefFile)
		if err := os.WriteFile(hashFile, []byte(outputFileName), 0644); err != nil {
			return fmt.Errorf("error writing to file: %v", err)
		}
	}

	if subDir == "critical" {
		concatenatedCSSString = naiveCSSMinify(concatenatedCSS.string())
	}

	return os.WriteFile(outputFile, []byte(concatenatedCSSString), 0644)
}

type staticFileProcessorOpts struct {
	dirName          string
	mapName          string
	shouldBeGranular bool
	getIsNoHashDir   func(string) bool
	writeWithHash    bool
}

func (c *Config) handlePublicFiles(shouldBeGranular bool) error {
	return c.processStaticFiles(&staticFileProcessorOpts{
		dirName:          publicDir,
		mapName:          PublicFileMapGobName,
		shouldBeGranular: shouldBeGranular,
		getIsNoHashDir: func(path string) bool {
			return strings.HasPrefix(path, "__nohash/")
		},
		writeWithHash: true,
	})
}

func (c *Config) copyPrivateFiles(shouldBeGranular bool) error {
	return c.processStaticFiles(&staticFileProcessorOpts{
		dirName:          privateDir,
		mapName:          PrivateFileMapGobName,
		shouldBeGranular: shouldBeGranular,
		getIsNoHashDir: func(path string) bool {
			return false
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
	cleanRootDir := c.getCleanRootDir()
	srcDir := filepath.Join(cleanRootDir, staticDir, opts.dirName)
	distDir := filepath.Join(cleanRootDir, distKirunaDir, staticDir, opts.dirName)

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil
	}

	newFileMap := &syncMap{m: make(map[string]string)}
	oldFileMap := &syncMap{m: make(map[string]string)}

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
		err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				relativePath, err := filepath.Rel(srcDir, path)
				if err != nil {
					return err
				}
				relativePath = filepath.ToSlash(relativePath)
				isNoHashDir := opts.getIsNoHashDir(relativePath)
				if isNoHashDir {
					relativePath = strings.TrimPrefix(relativePath, "__nohash/")
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
				if err := c.processFile(fi, opts, newFileMap, oldFileMap, distDir); err != nil {
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
		for k, v := range oldFileMap.m {
			if newHash, exists := newFileMap.Load(k); !exists || newHash != v {
				oldDistPath := filepath.Join(distDir, v)
				err := os.Remove(oldDistPath)
				if err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("error removing old static file from dist (%s/%s): %v", opts.dirName, v, err)
				}
			}
		}
	}

	// Save the updated file map
	err := c.saveMapToGob(newFileMap.m, opts.mapName)
	if err != nil {
		return fmt.Errorf("error saving file map: %v", err)
	}

	return nil
}

func (c *Config) processFile(fi fileInfo, opts *staticFileProcessorOpts, newFileMap, oldFileMap *syncMap, distDir string) error {
	if err := fileSemaphore.Acquire(context.Background(), 1); err != nil {
		return fmt.Errorf("error acquiring semaphore: %v", err)
	}
	defer fileSemaphore.Release(1)

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
