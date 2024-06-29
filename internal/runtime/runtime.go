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
	"github.com/sjc5/kiruna/internal/util"
	"github.com/sjc5/kit/pkg/executil"
	"github.com/sjc5/kit/pkg/fsutil"
	"github.com/sjc5/kit/pkg/typed"
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

var cacheMap = typed.SyncMap[*common.Config, *criticalCSSStatus]{}

func GetCriticalCSS(config *common.Config) string {
	// If cache hit and PROD, return hit
	if hit, isCached := cacheMap.Load(config); isCached && !common.KirunaEnv.GetIsDev() {
		hit.mu.RLock()
		defer hit.mu.RUnlock()
		if hit.noSuchFile {
			return ""
		}
		return hit.codeStr
	}

	// Instantiate cache or get existing
	cachedStatus, _ := cacheMap.LoadOrStore(config, &criticalCSSStatus{})

	// Get FS
	fs, err := GetUniversalFS(config)
	if err != nil {
		util.Log.Errorf("error getting FS: %v", err)
		return ""
	}

	// Read critical CSS
	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	content, err := fs.ReadFile(filepath.Join("internal", "critical.css"))
	if err != nil {
		// Check if the error is a non-existent file
		noSuchFile := strings.HasSuffix(err.Error(), "no such file or directory")

		cachedStatus.mu.Lock()
		cachedStatus.noSuchFile = noSuchFile // Set noSuchFile flag in cache
		cachedStatus.mu.Unlock()

		// if the error was something other than a non-existent file, log it
		if !noSuchFile {
			util.Log.Errorf("error reading critical CSS: %v", err)
		}
		return ""
	}

	criticalCSS := string(content)

	cachedStatus.mu.Lock()
	cachedStatus.codeStr = criticalCSS // Cache the critical CSS
	cachedStatus.mu.Unlock()

	return criticalCSS
}

func GetCriticalCSSStyleElement(config *common.Config) template.HTML {
	// If cache hit and PROD, return hit
	if hit, isCached := cacheMap.Load(config); isCached && !common.KirunaEnv.GetIsDev() {
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
	cached, _ := cacheMap.Load(config)

	cached.mu.RLock()
	noSuchFile := cached.noSuchFile
	cached.mu.RUnlock()

	// At this point, noSuchFile will have been set by GetCriticalCSS call above
	if noSuchFile {
		return ""
	}

	// Create style element
	el := template.HTML(fmt.Sprintf("<style id=\"%s\">%s</style>", CriticalCSSElementID, css))

	cached.mu.Lock()
	cached.styleEl = el           // Cache the element
	cached.styleElIsCached = true // Set element as cached
	cached.mu.Unlock()

	return el
}

////////////////////////////////////////////////////////////////////////////////
/////// GET PUBLIC URL
////////////////////////////////////////////////////////////////////////////////

var (
	fileMapFromGlobCacheMap = typed.SyncMap[string, map[string]string]{}
	fileMapLoadOnce         = typed.SyncMap[string, *sync.Once]{}
	urlCacheMap             = typed.SyncMap[string, string]{}
)

func GetPublicURL(config *common.Config, originalPublicURL string, useDirFS bool) string {
	fileMapKey := fmt.Sprintf("%p", config) + fmt.Sprintf("%t", useDirFS)
	urlKey := fileMapKey + originalPublicURL

	if hit, isCached := urlCacheMap.Load(urlKey); isCached {
		return hit
	}

	once, _ := fileMapLoadOnce.LoadOrStore(fileMapKey, &sync.Once{})
	once.Do(func() {
		fileMapFromGob, err := LoadMapFromGob(config, common.PublicFileMapGobName, useDirFS)
		if err != nil {
			util.Log.Errorf("error loading file map from gob: %v", err)
			return
		}
		fileMapFromGlobCacheMap.Store(fileMapKey, fileMapFromGob)
	})

	fileMap, _ := fileMapFromGlobCacheMap.Load(fileMapKey)
	if fileMap == nil {
		return originalPublicURL
	}

	if hashedURL, existsInFileMap := fileMap[cleanURL(originalPublicURL)]; existsInFileMap {
		finalURL := "/public/" + hashedURL
		urlCacheMap.Store(urlKey, finalURL) // Cache the hashed URL
		return finalURL
	}

	// If no hashed URL found, return the original URL
	util.Log.Infof(
		"GetPublicURL: no hashed URL found for %s, returning original URL",
		originalPublicURL,
	)
	finalURL := "/public/" + originalPublicURL
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
	FS, err := GetFS(config, "public")
	if err != nil {
		util.Log.Errorf("error getting public FS: %v", err)
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
		util.Log.Errorf("error getting FS: %v", err)
		return ""
	}

	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	content, err := fs.ReadFile(filepath.Join("internal", "normal_css_file_ref.txt"))
	if err != nil {
		util.Log.Errorf("error reading normal CSS URL: %v", err)
		return ""
	}

	url := "/public/" + string(content)
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

	el := template.HTML(fmt.Sprintf(
		"<link rel=\"stylesheet\" href=\"%s\" id=\"%s\" />", url, StyleSheetElementID,
	))
	styleSheetElementCacheMap.Store(config, el) // Cache the element
	return el
}

////////////////////////////////////////////////////////////////////////////////
/////// LOAD MAP FROM GOB
////////////////////////////////////////////////////////////////////////////////

func LoadMapFromGob(config *common.Config, gobFileName string, useDirFS bool) (map[string]string, error) {
	var FS *UniversalFS
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
	file, err := FS.Open(filepath.Join("internal", gobFileName))
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

type UniversalFS struct {
	FS fs.FS
}

func newUniversalFS(fs fs.FS) *UniversalFS {
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

func (u *UniversalFS) Sub(dir string) (*UniversalFS, error) {
	subFS, err := fs.Sub(u.FS, dir)
	if err != nil {
		return nil, err
	}
	FS := newUniversalFS(subFS)
	return FS, nil
}

var uniFSCacheMap = typed.SyncMap[*common.Config, *UniversalFS]{}

const fsTypeDev = "dev"

var fsTypeCacheMap = typed.SyncMap[*common.Config, string]{}

func GetUniversalFS(config *common.Config) (*UniversalFS, error) {
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

		util.Log.Infof("using disk file system (development)")
		fs := newUniversalFS(os.DirFS(path.Join(config.GetCleanRootDir(), "dist/kiruna")))
		actualFS, _ := uniFSCacheMap.LoadOrStore(config, fs) // cache the fs
		return actualFS, nil
	}

	// PROD
	// If we are using the embedded file system, we should use the dist file system
	if config.GetIsUsingEmbeddedFS() {
		util.Log.Infof("using embedded file system (production)")

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
	util.Log.Infof("using disk file system (production)")
	execDir, err := executil.GetExecutableDir()
	if err != nil {
		return nil, err
	}
	fs := newUniversalFS(os.DirFS(execDir))
	actualFS, _ := uniFSCacheMap.LoadOrStore(config, fs) // cache the fs
	return actualFS, nil
}

func GetFS(config *common.Config, subDir string) (*UniversalFS, error) {
	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	path := filepath.Join("static", subDir)

	FS, err := GetUniversalFS(config)
	if err != nil {
		errMsg := fmt.Sprintf("error getting %s FS: %v", subDir, err)
		util.Log.Errorf(errMsg)
		return nil, errors.New(errMsg)
	}
	subFS, err := FS.Sub(path)
	if err != nil {
		errMsg := fmt.Sprintf("error getting %s FS: %v", subDir, err)
		util.Log.Errorf(errMsg)
		return nil, errors.New(errMsg)
	}
	return subFS, nil
}

var uniDirFSCacheMap = typed.SyncMap[*common.Config, *UniversalFS]{}

func GetUniversalDirFS(config *common.Config) *UniversalFS {
	if hit, isCached := uniDirFSCacheMap.Load(config); isCached {
		return hit
	}
	fs := newUniversalFS(os.DirFS(path.Join(config.GetCleanRootDir(), "dist/kiruna")))
	actualFS, _ := uniDirFSCacheMap.LoadOrStore(config, fs)
	return actualFS
}
