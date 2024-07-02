package kiruna

import (
	"html/template"
	"net/http"

	ik "github.com/sjc5/kiruna/internal/kiruna"
)

type Config = ik.Config
type DevConfig = ik.DevConfig

type Kiruna struct {
	Config *ik.Config
}

// If you want to do a custom build command, just use
// Kiruna.BuildWithoutCompilingGo() instead of Kiruna.Build(),
// and then you can control your build yourself afterwards.

func (k Kiruna) Build() error {
	return k.Config.Build(true, false)
}
func (k Kiruna) BuildWithoutCompilingGo() error {
	return k.Config.Build(false, false)
}

func (k Kiruna) GetPublicFS() (ik.UniversalFS, error) {
	return k.Config.GetPublicFS()
}
func (k Kiruna) GetPrivateFS() (ik.UniversalFS, error) {
	return k.Config.GetPrivateFS()
}
func (k Kiruna) GetPublicURL(originalPublicURL string) string {
	return k.Config.GetPublicURL(originalPublicURL, false)
}

/*
 * MakePublicURLsMap creates a map of public URLs for the given filepaths, where
 * the keys are sanitized versions of the filepaths, replacing non-alphanumeric
 * characters with underscores. For example, the filepath "javascript/main.js"
 * would be sanitized to "javascript_main_js". One use case for this is to
 * pass a map of public URLs to a Go template.
 */
func (k Kiruna) MakePublicURLsMap(filepaths []string) map[string]string {
	return k.Config.MakePublicURLsMap(filepaths, false)
}
func (k Kiruna) MustStartDev(devConfig *ik.DevConfig) {
	k.Config.DevConfig = devConfig
	k.Config.MustStartDev()
}
func (k Kiruna) GetCriticalCSS() template.CSS {
	return template.CSS(k.Config.GetCriticalCSS())
}
func (k Kiruna) GetStyleSheetURL() string {
	return k.Config.GetStyleSheetURL()
}
func (k Kiruna) GetRefreshScript() template.HTML {
	return template.HTML(ik.GetRefreshScript(k.Config))
}
func (k Kiruna) GetCriticalCSSElementID() string {
	return ik.CriticalCSSElementID
}
func (k Kiruna) GetStyleSheetElementID() string {
	return ik.StyleSheetElementID
}
func (k Kiruna) GetUniversalFS() (ik.UniversalFS, error) {
	return k.Config.GetUniversalFS()
}
func (k Kiruna) GetCriticalCSSStyleElement() template.HTML {
	return k.Config.GetCriticalCSSStyleElement()
}
func (k Kiruna) GetStyleSheetLinkElement() template.HTML {
	return k.Config.GetStyleSheetLinkElement()
}
func (k Kiruna) GetServeStaticHandler(pathPrefix string, cacheImmutably bool) http.Handler {
	return k.Config.GetServeStaticHandler(pathPrefix, cacheImmutably)
}
func (k Kiruna) GetPublicFileMapElements() template.HTML {
	return template.HTML(k.Config.GetPublicFileMapElements())
}

func New(config *ik.Config) *Kiruna {
	if config.Logger == nil {
		config.Logger = &ik.Log
	}
	return &Kiruna{
		Config: config,
	}
}

type WatchedFile = ik.WatchedFile
type WatchedFiles = ik.WatchedFiles
type OnChangeFunc = ik.OnChangeFunc
type OnChange = ik.OnChange
type IgnorePatterns = ik.IgnorePatterns
type UniversalFS = ik.UniversalFS

const OnChangeStrategyConcurrent = ik.OnChangeStrategyConcurrent
const OnChangeStrategyPost = ik.OnChangeStrategyPost
const OnChangeStrategyPre = ik.OnChangeStrategyPre
const OnChangeStrategyConcurrentNoWait = ik.OnChangeStrategyConcurrentNoWait

var SetupDistDir = ik.SetupDistDir
var MustGetPort = ik.MustGetPort
var GetIsDev = ik.KirunaEnv.GetIsDev
