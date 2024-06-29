package kiruna

import (
	"html/template"
	"net/http"

	"github.com/sjc5/kiruna/internal/buildtime"
	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/dev"
	"github.com/sjc5/kiruna/internal/runtime"
	"github.com/sjc5/kiruna/internal/util"
)

type Config = common.Config
type DevConfig = common.DevConfig

type Kiruna struct {
	Config *common.Config
}

func (k Kiruna) Build() error {
	return buildtime.Build(k.Config, true, false)
}
func (k Kiruna) BuildWithoutCompilingGo() error {
	return buildtime.Build(k.Config, false, false)
}
func (k Kiruna) GetPublicFS() (runtime.UniversalFSInterface, error) {
	return runtime.GetFS(k.Config, "public")
}
func (k Kiruna) GetPrivateFS() (runtime.UniversalFSInterface, error) {
	return runtime.GetFS(k.Config, "private")
}
func (k Kiruna) GetPublicURL(originalPublicURL string) string {
	return runtime.GetPublicURL(k.Config, originalPublicURL, false)
}

/*
 * MakePublicURLsMap creates a map of public URLs for the given filepaths, where
 * the keys are sanitized versions of the filepaths, replacing non-alphanumeric
 * characters with underscores. For example, the filepath "javascript/main.js"
 * would be sanitized to "javascript_main_js". One use case for this is to
 * pass a map of public URLs to a Go template.
 */
func (k Kiruna) MakePublicURLsMap(filepaths []string) map[string]string {
	return runtime.MakePublicURLsMap(k.Config, filepaths, false)
}
func (k Kiruna) MustStartDev(devConfig *common.DevConfig) {
	k.Config.DevConfig = devConfig
	dev.MustStartDev(k.Config)
}
func (k Kiruna) GetCriticalCSS() template.CSS {
	return template.CSS(runtime.GetCriticalCSS(k.Config))
}
func (k Kiruna) GetStyleSheetURL() string {
	return runtime.GetStyleSheetURL(k.Config)
}
func (k Kiruna) GetRefreshScript() template.HTML {
	return template.HTML(runtime.GetRefreshScript(k.Config))
}
func (k Kiruna) GetCriticalCSSElementID() string {
	return runtime.CriticalCSSElementID
}
func (k Kiruna) GetStyleSheetElementID() string {
	return runtime.StyleSheetElementID
}
func (k Kiruna) GetUniversalFS() (runtime.UniversalFSInterface, error) {
	return runtime.GetUniversalFS(k.Config)
}
func (k Kiruna) GetCriticalCSSStyleElement() template.HTML {
	return runtime.GetCriticalCSSStyleElement(k.Config)
}
func (k Kiruna) GetStyleSheetLinkElement() template.HTML {
	return runtime.GetStyleSheetLinkElement(k.Config)
}
func (k Kiruna) GetServeStaticHandler(pathPrefix string, cacheImmutably bool) http.Handler {
	return runtime.GetServeStaticHandler(k.Config, pathPrefix, cacheImmutably)
}

func New(config *common.Config) *Kiruna {
	if config.Logger == nil {
		config.Logger = &util.Log
	}
	return &Kiruna{
		Config: config,
	}
}

type WatchedFile = common.WatchedFile
type WatchedFiles = common.WatchedFiles
type OnChangeFunc = common.OnChangeFunc
type OnChange = common.OnChange
type IgnorePatterns = common.IgnorePatterns
type UniversalFS = runtime.UniversalFS

const OnChangeStrategyConcurrent = common.OnChangeStrategyConcurrent
const OnChangeStrategyPost = common.OnChangeStrategyPost
const OnChangeStrategyPre = common.OnChangeStrategyPre
const OnChangeStrategyConcurrentNoWait = common.OnChangeStrategyConcurrentNoWait

var SetupDistDir = buildtime.SetupDistDir
var MustGetPort = util.MustGetPort
var GetIsDev = common.KirunaEnv.GetIsDev
