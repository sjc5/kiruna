package kiruna

import (
	"html/template"
	"net/http"

	ik "github.com/sjc5/kiruna/internal/kiruna"
	"github.com/sjc5/kit/pkg/colorlog"
)

type Config = ik.Config
type DevConfig = ik.DevConfig

type Kiruna struct {
	c *Config
}

// If you want to do a custom build command, just use
// Kiruna.BuildWithoutCompilingGo() instead of Kiruna.Build(),
// and then you can control your build yourself afterwards.

func (k Kiruna) Build() error {
	return k.c.Build(true, false)
}
func (k Kiruna) BuildWithoutCompilingGo() error {
	return k.c.Build(false, false)
}

func (k Kiruna) GetPublicFS() (UniversalFS, error) {
	return k.c.GetPublicFS()
}
func (k Kiruna) GetPrivateFS() (UniversalFS, error) {
	return k.c.GetPrivateFS()
}
func (k Kiruna) GetPublicURL(originalPublicURL string) string {
	return k.c.GetPublicURL(originalPublicURL)
}

/*
 * MakePublicURLsMap creates a map of public URLs for the given filepaths, where
 * the keys are sanitized versions of the filepaths, replacing non-alphanumeric
 * characters with underscores. For example, the filepath "javascript/main.js"
 * would be sanitized to "javascript_main_js". One use case for this is to
 * pass a map of public URLs to a Go template.
 */
func (k Kiruna) MakePublicURLsMap(filepaths []string) map[string]string {
	return k.c.MakePublicURLsMap(filepaths)
}
func (k Kiruna) MustStartDev(devConfig *DevConfig) {
	k.c.DevConfig = devConfig
	k.c.MustStartDev()
}
func (k Kiruna) GetCriticalCSS() template.CSS {
	return template.CSS(k.c.GetCriticalCSS())
}
func (k Kiruna) GetStyleSheetURL() string {
	return k.c.GetStyleSheetURL()
}
func (k Kiruna) GetRefreshScript() template.HTML {
	return template.HTML(k.c.GetRefreshScript())
}
func (k Kiruna) GetCriticalCSSElementID() string {
	return ik.CriticalCSSElementID
}
func (k Kiruna) GetStyleSheetElementID() string {
	return ik.StyleSheetElementID
}
func (k Kiruna) GetUniversalFS() (UniversalFS, error) {
	return k.c.GetUniversalFS()
}
func (k Kiruna) GetCriticalCSSStyleElement() template.HTML {
	return k.c.GetCriticalCSSStyleElement()
}
func (k Kiruna) GetStyleSheetLinkElement() template.HTML {
	return k.c.GetStyleSheetLinkElement()
}
func (k Kiruna) GetServeStaticHandler(pathPrefix string, cacheImmutably bool) http.Handler {
	return k.c.GetServeStaticHandler(pathPrefix, cacheImmutably)
}
func (k Kiruna) GetPublicFileMapElements() template.HTML {
	return template.HTML(k.c.GetPublicFileMapElements())
}
func (k Kiruna) GetPublicFileMapURL() string {
	return k.c.GetPublicFileMapURL()
}

func New(c *ik.Config) *Kiruna {
	if c.Logger == nil {
		c.Logger = &colorlog.Log{Label: "Kiruna"}
	}
	c.RuntimeInitOnce()
	return &Kiruna{c}
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
