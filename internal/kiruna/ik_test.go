package ik

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sjc5/kit/pkg/colorlog"
	"github.com/sjc5/kit/pkg/safecache"
	"golang.org/x/sync/semaphore"
)

const testRootDir = "testdata"

// testEnv holds our testing environment
type testEnv struct {
	config *Config
}

// setupTestEnv creates a new test environment
func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	// Set up the source directory structure
	sourceDirs := []string{
		"styles/critical",
		"styles/normal",
		"static/public",
		"static/private",
	}

	// Set up the dist directory structure
	distDirs := []string{
		"dist/kiruna/static/public",
		"dist/kiruna/static/private",
		"dist/kiruna/internal",
	}

	for _, dir := range append(sourceDirs, distDirs...) {
		if err := os.MkdirAll(filepath.Join(testRootDir, dir), 0755); err != nil {
			t.Fatalf("Failed to create directory structure: %v", err)
		}
	}

	config := &Config{
		PrivateStaticDir: filepath.Join(testRootDir, "static", "private"),
		PublicStaticDir:  filepath.Join(testRootDir, "static", "public"),
		StylesDir:        filepath.Join(testRootDir, "styles"),
		DistDir:          filepath.Join(testRootDir, "dist"),
		MainAppEntry:     "cmd/app/main.go",
		Logger:           &colorlog.Log{},
	}

	// Initialize the fileSemaphore
	config.fileSemaphore = semaphore.NewWeighted(100)

	// Set up embedded FS
	config.DistFS = os.DirFS(filepath.Join(testRootDir, "dist"))

	// Initialize safecache
	config.runtime.cache = runtimeCache{
		uniFS:                 safecache.New(config.getInitialUniversalFS, nil),
		uniDirFS:              safecache.New(config.getInitialUniversalDirFS, nil),
		publicFS:              safecache.New(func() (UniversalFS, error) { return config.getFS(publicDir) }, nil),
		privateFS:             safecache.New(func() (UniversalFS, error) { return config.getFS(privateDir) }, nil),
		styleSheetLinkElement: safecache.New(config.getInitialStyleSheetLinkElement, getIsDev),
		styleSheetURL:         safecache.New(config.getInitialStyleSheetURL, getIsDev),
		criticalCSS:           safecache.New(config.getInitialCriticalCSSStatus, getIsDev),
		publicFileMapFromGob:  safecache.New(config.getInitialPublicFileMapFromGobRuntime, nil),
		publicFileMapURL:      safecache.New(config.getInitialPublicFileMapURL, getIsDev),
		publicURLs:            safecache.NewMap(config.getInitialPublicURL, publicURLsKeyMaker, nil),
	}

	// Initialize dev cache if needed
	config.dev.matchResults = safecache.NewMap(config.getInitialMatchResults, config.matchResultsKeyMaker, nil)

	// Set to production mode for testing
	os.Setenv(modeKey, "production")

	return &testEnv{
		config: config,
	}
}

// teardownTestEnv cleans up the test environment
func teardownTestEnv(t *testing.T) {
	t.Helper()

	if err := os.RemoveAll(testRootDir); err != nil {
		t.Errorf("Failed to remove test directory: %v", err)
	}

	// Reset environment variables
	os.Unsetenv(modeKey)
}

// createTestFile creates a file with given content in the test environment
func (env *testEnv) createTestFile(t *testing.T, relativePath, content string) {
	t.Helper()

	fullPath := filepath.Join(testRootDir, relativePath)
	dir := filepath.Dir(fullPath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create directory %s: %v", dir, err)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file %s: %v", fullPath, err)
	}
}

// resetEnv resets environment variables to a known state
func resetEnv() {
	os.Unsetenv(modeKey)
	os.Unsetenv(portKey)
	os.Unsetenv(portHasBeenSetKey)
	os.Unsetenv(refreshServerPortKey)
	os.Unsetenv(isBuildTimeKey)
}

func TestMain(m *testing.M) {
	code := m.Run()
	os.RemoveAll(testRootDir)
	os.Exit(code)
}
