package ik

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

func (c *Config) compileBinary() error {
	cleanRootDir := c.getCleanRootDir()
	buildDest := filepath.Join(cleanRootDir, "dist/bin/main")
	entryPoint := filepath.Join(cleanRootDir, c.EntryPoint)
	buildCmd := exec.Command("go", "build", "-o", buildDest, entryPoint)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	err := buildCmd.Run()
	if err != nil {
		return fmt.Errorf("error compiling binary: %v", err)
	}
	c.Logger.Infof("compilation complete: %s", buildDest)
	return nil
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
		err = os.RemoveAll(filepath.Join(cleanRootDir, distKirunaDir))
		if err != nil {
			return fmt.Errorf("error removing dist/kiruna directory: %v", err)
		}

		// re-make required directories
		isServerOnly := c.DevConfig != nil && c.DevConfig.ServerOnly
		if !isServerOnly {
			err = SetupDistDir(c.RootDir)
			if err != nil {
				return fmt.Errorf("error making requisite directories: %v", err)
			}
		}

		// add pid file back
		if lastPID != 0 {
			err = pidFile.writePIDFile(lastPID)
			if err != nil {
				return fmt.Errorf("error writing PID file: %v", err)
			}
		}
	}

	// Must be complete before BuildCSS in case the CSS references any public files
	err := c.handlePublicFiles(shouldBeGranular)
	if err != nil {
		return fmt.Errorf("error handling public files: %v", err)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 2) // Buffer to hold up to 2 errors
	wg.Add(2)                      // Two tasks to do concurrently

	// goroutine 1
	go func() {
		defer wg.Done()
		if err = c.copyPrivateFiles(shouldBeGranular); err != nil {
			errChan <- precompileError{task: "copyPrivateFiles", err: err}
		}
	}()

	// goroutine 2
	go func() {
		defer wg.Done()
		if err = c.BuildCSS(); err != nil {
			errChan <- precompileError{task: "BuildCSS", err: err}
		}
	}()

	// Wait for all tasks to complete
	wg.Wait()
	close(errChan)

	// Check for errors
	for e := range errChan {
		if e != nil {
			return e
		}
	}

	if recompileBinary {
		err = c.compileBinary()
		if err != nil {
			return fmt.Errorf("error compiling binary: %v", err)
		}
	}
	return nil
}

func (c *Config) BuildCSS() error {
	err := c.ProcessCSS("critical")
	if err != nil {
		return fmt.Errorf("error processing critical CSS: %v", err)
	}

	err = c.ProcessCSS("normal")
	if err != nil {
		return fmt.Errorf("error processing normal CSS: %v", err)
	}

	return nil
}

var urlRegex = regexp.MustCompile(`url\(([^)]+)\)`)

// ProcessCSS concatenates and hashes specified CSS files, then saves them to disk.
func (c *Config) ProcessCSS(subDir string) error {
	cleanRootDir := c.getCleanRootDir()

	dirPath := filepath.Join(cleanRootDir, "styles", subDir)
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return nil
	}
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("error reading directory: %v", err)
	}

	var (
		concatenatedCSS strings.Builder
		fileNames       []string
	)

	// Collect and sort .css files
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".css") {
			fileNames = append(fileNames, file.Name())
		}
	}
	sort.Strings(fileNames)

	// Concatenate file contents
	for _, fileName := range fileNames {
		content, err := os.ReadFile(filepath.Join(dirPath, fileName))
		if err != nil {
			return fmt.Errorf("error reading file: %v", err)
		}
		concatenatedCSS.Write(content)
	}

	concatenatedCSSString := concatenatedCSS.String()
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
		outputFileName = getHashedFilename(
			[]byte(concatenatedCSS.String()),
			"normal.css",
		)
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
		concatenatedCSSString = naiveCSSMinify(concatenatedCSS.String())
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

func (c *Config) processStaticFiles(opts *staticFileProcessorOpts) error {
	cleanRootDir := c.getCleanRootDir()
	srcDir := filepath.Join(cleanRootDir, staticDir, opts.dirName)
	distDir := filepath.Join(cleanRootDir, distKirunaDir, staticDir, opts.dirName)

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil
	}

	newFileMap := make(map[string]string)
	oldFileMap := make(map[string]string)

	// Load old file map if granular updates are enabled
	if opts.shouldBeGranular {
		var err error
		oldFileMap, err = c.LoadMapFromGob(opts.mapName, true)
		if err != nil {
			return fmt.Errorf("error reading old file map: %v", err)
		}
	}

	// Walk the directory and process files
	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error walking dir: %v", err)
		}
		if d.IsDir() {
			return nil
		}

		relativePath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("error getting relative path: %v", err)
		}

		// Normalize path
		relativePath = filepath.ToSlash(relativePath)
		isNoHashDir := opts.getIsNoHashDir(relativePath)
		if isNoHashDir {
			relativePath = strings.TrimPrefix(relativePath, "__nohash/")
		}
		relativePathUnderscores := strings.ReplaceAll(relativePath, "/", "_")

		var contentBytes []byte
		hasSetContentBytes := false

		var fileIdentifier string
		if isNoHashDir {
			fileIdentifier = relativePath
		} else {
			contentBytes, err = os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("error reading file: %v", err)
			}
			hasSetContentBytes = true

			fileIdentifier = getHashedFilename(contentBytes, relativePathUnderscores)
		}

		newFileMap[relativePath] = fileIdentifier

		// Skip unchanged files if granular updates are enabled
		if opts.shouldBeGranular {
			if oldHash, exists := oldFileMap[relativePath]; exists && oldHash == fileIdentifier {
				return nil
			}
		}

		var distPath string
		if opts.writeWithHash {
			distPath = filepath.Join(distDir, fileIdentifier)
		} else {
			distPath = filepath.Join(distDir, relativePath)
		}

		err = os.MkdirAll(filepath.Dir(distPath), 0755)
		if err != nil {
			return fmt.Errorf("error creating directory: %v", err)
		}

		if !hasSetContentBytes {
			contentBytes, err = os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("error reading file: %v", err)
			}
			hasSetContentBytes = true // not strictly needed, but trying to be nice to my future self
		}

		err = os.WriteFile(distPath, contentBytes, 0644)
		if err != nil {
			return fmt.Errorf("error writing file: %v", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("error walking dir: %v", err)
	}

	// Cleanup old moot files if granular updates are enabled
	if opts.shouldBeGranular {
		for relativePath, oldHash := range oldFileMap {
			newHash := newFileMap[relativePath]

			if oldHash != newHash {
				oldDistPath := filepath.Join(distDir, oldHash)
				err := os.Remove(oldDistPath)
				if err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("error removing old static file from dist (%s/%s): %v", opts.dirName, oldHash, err)
				}
			}
		}
	}

	// Save the updated file map
	err = saveMapToGob(cleanRootDir, newFileMap, opts.mapName)
	if err != nil {
		return fmt.Errorf("error saving file map: %v", err)
	}

	return nil
}

func getHashedFilename(content []byte, originalFileName string) string {
	hash := sha256.New()
	hash.Write(content)
	hashedSuffix := fmt.Sprintf("%x", hash.Sum(nil))[:12] // Short hash
	ext := filepath.Ext(originalFileName)
	outputFileName := fmt.Sprintf("%s_%s%s", strings.TrimSuffix(originalFileName, ext), hashedSuffix, ext)
	return outputFileName
}

func SetupDistDir(rootDir string) error {
	cleanRootDir := filepath.Clean(rootDir)

	// make a dist/kiruna/internal directory
	path := filepath.Join(cleanRootDir, distKirunaDir, internalDir)
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("error making internal directory: %v", err)
	}

	// add a x file so that go:embed doesn't complain
	path = filepath.Join(cleanRootDir, distKirunaDir, "x")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		return fmt.Errorf("error making x file: %v", err)
	}

	// need an empty dist/kiruna/public directory
	path = filepath.Join(cleanRootDir, distKirunaDir, staticDir, publicDir)
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("error making public directory: %v", err)
	}

	// need an empty dist/kiruna/private directory
	path = filepath.Join(cleanRootDir, distKirunaDir, staticDir, privateDir)
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("error making private directory: %v", err)
	}

	return nil
}

// Define a custom error type for more specific error handling
type precompileError struct {
	task string
	err  error
}

func (e precompileError) Error() string {
	return fmt.Sprintf("error during precompile task %s: %v", e.task, e.err)
}
