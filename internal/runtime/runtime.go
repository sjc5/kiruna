package runtime

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kit/pkg/executil"
	"github.com/sjc5/kit/pkg/fsutil"
	"github.com/sjc5/kit/pkg/typed"
)

const (
	internalDir          = "internal"
	publicDir            = "public"
	staticDir            = "static"
	distKirunaDir        = "dist/kiruna"
	criticalCSSFile      = "critical.css"
	normalCSSFileRefFile = "normal_css_file_ref.txt"
)

////////////////////////////////////////////////////////////////////////////////
/////// GET CRITICAL CSS
////////////////////////////////////////////////////////////////////////////////

const CriticalCSSElementID = "__critical-css"

type criticalCSSStatus struct {
	mu              sync.RWMutex
	codeStr         string
	noSuchFile      bool
	styleEl         template.HTML
	styleElIsCached bool
}

var criticalCSSCacheMap = typed.SyncMap[*common.Config, *criticalCSSStatus]{}

func GetCriticalCSS(config *common.Config) string {
	// If cache hit and PROD, return hit
	if hit, isCached := criticalCSSCacheMap.Load(config); isCached && !common.KirunaEnv.GetIsDev() {
		hit.mu.RLock()
		defer hit.mu.RUnlock()
		if hit.noSuchFile {
			return ""
		}
		return hit.codeStr
	}

	// Instantiate cache or get existing
	cachedStatus, _ := criticalCSSCacheMap.LoadOrStore(config, &criticalCSSStatus{})

	// Get FS
	fs, err := GetUniversalFS(config)
	if err != nil {
		config.Logger.Errorf("error getting FS: %v", err)
		return ""
	}

	// Read critical CSS
	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	content, err := fs.ReadFile(filepath.Join(internalDir, criticalCSSFile))
	if err != nil {
		cachedStatus.mu.Lock()
		defer cachedStatus.mu.Unlock()
		// Check if the error is a non-existent file, and set the noSuchFile flag in the cache
		cachedStatus.noSuchFile = strings.HasSuffix(err.Error(), "no such file or directory")

		// if the error was something other than a non-existent file, log it
		if !cachedStatus.noSuchFile {
			config.Logger.Errorf("error reading critical CSS: %v", err)
		}
		return ""
	}

	criticalCSS := string(content)

	cachedStatus.mu.Lock()
	defer cachedStatus.mu.Unlock()
	cachedStatus.codeStr = criticalCSS // Cache the critical CSS

	return criticalCSS
}

func GetCriticalCSSStyleElement(config *common.Config) template.HTML {
	// If cache hit and PROD, return hit
	if hit, isCached := criticalCSSCacheMap.Load(config); isCached && !common.KirunaEnv.GetIsDev() {
		hit.mu.RLock()
		defer hit.mu.RUnlock()
		if hit.noSuchFile {
			return ""
		}
		if hit.styleElIsCached {
			return hit.styleEl
		}
	}

	// Get critical CSS
	css := GetCriticalCSS(config)
	cached, _ := criticalCSSCacheMap.Load(config)

	cached.mu.RLock()
	noSuchFile := cached.noSuchFile
	cached.mu.RUnlock()

	// At this point, noSuchFile will have been set by GetCriticalCSS call above
	if noSuchFile {
		return ""
	}

	// Create style element
	var sb strings.Builder
	sb.WriteString(`<style id="`)
	sb.WriteString(CriticalCSSElementID)
	sb.WriteString(`">`)
	sb.WriteString(css)
	sb.WriteString("</style>")
	el := template.HTML(sb.String())

	cached.mu.Lock()
	defer cached.mu.Unlock()
	cached.styleEl = el           // Cache the element
	cached.styleElIsCached = true // Set element as cached

	return el
}

////////////////////////////////////////////////////////////////////////////////
/////// GET PUBLIC URL
////////////////////////////////////////////////////////////////////////////////

var (
	fileMapFromGobCacheMap = typed.SyncMap[string, map[string]string]{}
	fileMapLoadOnce        = typed.SyncMap[string, *sync.Once]{}
	urlCacheMap            = typed.SyncMap[string, string]{}
)

func GetPublicURL(config *common.Config, originalPublicURL string, useDirFS bool) string {
	fileMapKey := fmt.Sprintf("%p", config) + fmt.Sprintf("%t", useDirFS)
	urlKey := fileMapKey + originalPublicURL

	if hit, isCached := urlCacheMap.Load(urlKey); isCached {
		return hit
	}

	once, _ := fileMapLoadOnce.LoadOrStore(fileMapKey, &sync.Once{})
	var fileMapFromGob map[string]string
	once.Do(func() {
		var err error
		fileMapFromGob, err = LoadMapFromGob(config, common.PublicFileMapGobName, useDirFS)
		if err != nil {
			config.Logger.Errorf("error loading file map from gob: %v", err)
			return
		}
		fileMapFromGobCacheMap.Store(fileMapKey, fileMapFromGob)
	})

	if fileMapFromGob == nil {
		fileMapFromGob, _ = fileMapFromGobCacheMap.Load(fileMapKey)
	}
	if fileMapFromGob == nil {
		return originalPublicURL
	}

	if hashedURL, existsInFileMap := fileMapFromGob[cleanURL(originalPublicURL)]; existsInFileMap {
		finalURL := "/" + publicDir + "/" + hashedURL
		urlCacheMap.Store(urlKey, finalURL) // Cache the hashed URL
		return finalURL
	}

	// If no hashed URL found, return the original URL
	config.Logger.Infof(
		"GetPublicURL: no hashed URL found for %s, returning original URL",
		originalPublicURL,
	)
	finalURL := "/" + publicDir + "/" + originalPublicURL
	urlCacheMap.Store(urlKey, finalURL) // Cache the original URL
	return finalURL
}

func MakePublicURLsMap(config *common.Config, filepaths []string, useDirFS bool) map[string]string {
	filepathsMap := make(map[string]string, len(filepaths))
	var sb strings.Builder
	sb.Grow(64)

	for _, filepath := range filepaths {
		sb.Reset()
		for _, r := range filepath {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				sb.WriteRune(r)
			} else {
				sb.WriteRune('_')
			}
		}
		safeKey := sb.String()
		filepathsMap[safeKey] = GetPublicURL(config, filepath, useDirFS)
	}

	return filepathsMap
}

func cleanURL(url string) string {
	return strings.TrimPrefix(filepath.Clean(url), "/")
}

////////////////////////////////////////////////////////////////////////////////
/////// GET SERVE STATIC HANDLER
////////////////////////////////////////////////////////////////////////////////

func GetServeStaticHandler(config *common.Config, pathPrefix string, cacheImmutably bool) http.Handler {
	FS, err := GetFS(config, publicDir)
	if err != nil {
		config.Logger.Errorf("error getting public FS: %v", err)
	}
	if cacheImmutably {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			http.StripPrefix(pathPrefix, http.FileServer(http.FS(FS))).ServeHTTP(w, r)
		})
	}
	return http.StripPrefix(pathPrefix, http.FileServer(http.FS(FS)))
}

////////////////////////////////////////////////////////////////////////////////
/////// STYLESHEET
////////////////////////////////////////////////////////////////////////////////

const StyleSheetElementID = "__normal-css"

var styleSheetURLCacheMap = typed.SyncMap[*common.Config, string]{}

func GetStyleSheetURL(config *common.Config) string {
	if hit, isCached := styleSheetURLCacheMap.Load(config); isCached && !common.KirunaEnv.GetIsDev() {
		return hit
	}

	fs, err := GetUniversalFS(config)
	if err != nil {
		config.Logger.Errorf("error getting FS: %v", err)
		return ""
	}

	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	content, err := fs.ReadFile(filepath.Join(internalDir, normalCSSFileRefFile))
	if err != nil {
		config.Logger.Errorf("error reading normal CSS URL: %v", err)
		return ""
	}

	url := "/" + publicDir + "/" + string(content)
	styleSheetURLCacheMap.Store(config, url) // Cache the URL
	return url
}

var styleSheetElementCacheMap = typed.SyncMap[*common.Config, template.HTML]{}

func GetStyleSheetLinkElement(config *common.Config) template.HTML {
	if hit, isCached := styleSheetElementCacheMap.Load(config); isCached && !common.KirunaEnv.GetIsDev() {
		return hit
	}

	url := GetStyleSheetURL(config)
	if url == "" {
		styleSheetElementCacheMap.Store(config, "") // Cache the empty string
		return ""
	}

	var sb strings.Builder
	sb.WriteString(`<link rel="stylesheet" href="`)
	sb.WriteString(url)
	sb.WriteString(`" id="`)
	sb.WriteString(StyleSheetElementID)
	sb.WriteString(`" />`)
	el := template.HTML(sb.String())

	styleSheetElementCacheMap.Store(config, el) // Cache the element
	return el
}

////////////////////////////////////////////////////////////////////////////////
/////// LOAD MAP FROM GOB
////////////////////////////////////////////////////////////////////////////////

func LoadMapFromGob(config *common.Config, gobFileName string, useDirFS bool) (map[string]string, error) {
	var FS UniversalFSInterface
	var err error
	if useDirFS {
		FS = GetUniversalDirFS(config)
	} else {
		FS, err = GetUniversalFS(config)
	}
	if err != nil {
		return nil, fmt.Errorf("error getting FS: %v", err)
	}

	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	file, err := FS.Open(filepath.Join(internalDir, gobFileName))
	if err != nil {
		return nil, fmt.Errorf("error opening file: %v", err)
	}

	defer file.Close()

	var mapFromGob map[string]string
	err = fsutil.FromGobInto(file, &mapFromGob)
	if err != nil {
		return nil, fmt.Errorf("error decoding gob: %v", err)
	}
	return mapFromGob, nil
}

////////////////////////////////////////////////////////////////////////////////
/////// FS
////////////////////////////////////////////////////////////////////////////////

type UniversalFSInterface interface {
	ReadFile(name string) ([]byte, error)
	Open(name string) (fs.File, error)
	ReadDir(name string) ([]fs.DirEntry, error)
	Sub(dir string) (UniversalFSInterface, error)
}

type UniversalFS struct {
	FS fs.FS
}

func newUniversalFS(fs fs.FS) UniversalFSInterface {
	return &UniversalFS{FS: fs}
}

func (u *UniversalFS) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(u.FS, name)
}

func (u *UniversalFS) Open(name string) (fs.File, error) {
	return u.FS.Open(name)
}

func (u *UniversalFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(u.FS, name)
}

func (u *UniversalFS) Sub(dir string) (UniversalFSInterface, error) {
	subFS, err := fs.Sub(u.FS, dir)
	if err != nil {
		return nil, err
	}
	return newUniversalFS(subFS), nil
}

var uniFSCacheMap = typed.SyncMap[*common.Config, UniversalFSInterface]{}

const fsTypeDev = "dev"

var fsTypeCacheMap = typed.SyncMap[*common.Config, string]{}

func GetUniversalFS(config *common.Config) (UniversalFSInterface, error) {
	if hit, isCached := uniFSCacheMap.Load(config); isCached {
		cachedFSType, _ := fsTypeCacheMap.Load(config)
		skipCache := common.KirunaEnv.GetIsDev() && cachedFSType != fsTypeDev
		if !skipCache {
			return hit, nil
		}
	}

	// DEV
	// There is an expectation that you run the dev server from the root of your project,
	// where your go.mod file is.
	if common.KirunaEnv.GetIsDev() {
		// ensures "needsReset" is always true in dev
		fsTypeCacheMap.Store(config, fsTypeDev)

		config.Logger.Infof("using disk file system (development)")
		fs := newUniversalFS(os.DirFS(path.Join(config.GetCleanRootDir(), distKirunaDir)))
		actualFS, _ := uniFSCacheMap.LoadOrStore(config, fs) // cache the fs
		return actualFS, nil
	}

	// PROD
	// If we are using the embedded file system, we should use the dist file system
	if config.GetIsUsingEmbeddedFS() {
		config.Logger.Infof("using embedded file system (production)")

		// Assuming the embed directive looks like this:
		// //go:embed kiruna
		// That means that the kiruna folder itself (not just its contents) is embedded.
		// So we have to drop down into the kiruna folder here.
		FS, err := fs.Sub(config.DistFS, "kiruna")
		if err != nil {
			return nil, err
		}
		fs := newUniversalFS(FS)
		actualFS, _ := uniFSCacheMap.LoadOrStore(config, fs) // cache the fs
		return actualFS, nil
	}

	// PROD
	// If we are not using the embedded file system, we should use the os file system,
	// and assume that the executable is a sibling to the kiruna-outputted "kiruna" directory
	config.Logger.Infof("using disk file system (production)")
	execDir, err := executil.GetExecutableDir()
	if err != nil {
		return nil, err
	}
	fs := newUniversalFS(os.DirFS(execDir))
	actualFS, _ := uniFSCacheMap.LoadOrStore(config, fs) // cache the fs
	return actualFS, nil
}

func GetFS(config *common.Config, subDir string) (UniversalFSInterface, error) {
	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	path := filepath.Join(staticDir, subDir)

	FS, err := GetUniversalFS(config)
	if err != nil {
		errMsg := fmt.Sprintf("error getting %s FS: %v", subDir, err)
		config.Logger.Errorf(errMsg)
		return nil, errors.New(errMsg)
	}
	subFS, err := FS.Sub(path)
	if err != nil {
		errMsg := fmt.Sprintf("error getting %s FS: %v", subDir, err)
		config.Logger.Errorf(errMsg)
		return nil, errors.New(errMsg)
	}
	return subFS, nil
}

var uniDirFSCacheMap = typed.SyncMap[*common.Config, UniversalFSInterface]{}

func GetUniversalDirFS(config *common.Config) UniversalFSInterface {
	if hit, isCached := uniDirFSCacheMap.Load(config); isCached {
		return hit
	}
	fs := newUniversalFS(os.DirFS(path.Join(config.GetCleanRootDir(), distKirunaDir)))
	actualFS, _ := uniDirFSCacheMap.LoadOrStore(config, fs)
	return actualFS
}
