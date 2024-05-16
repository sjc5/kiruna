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
type CSSConfig = common.CSSConfig
type Extensions = common.Extensions

type Kiruna struct {
	Config *common.Config
}

func (k Kiruna) Build() error {
	return buildtime.Build(k.Config, true)
}
func (k Kiruna) BuildWithoutCompilingGo() error {
	return buildtime.Build(k.Config, false)
}
func (k Kiruna) GetPublicFS() (*runtime.UniversalFS, error) {
	return runtime.GetPublicFS(k.Config)
}
func (k Kiruna) GetPrivateFS() (*runtime.UniversalFS, error) {
	return runtime.GetPrivateFS(k.Config)
}
func (k Kiruna) GetPublicURL(originalPublicURL string) string {
	return runtime.GetPublicURL(k.Config, originalPublicURL, false)
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
func (k Kiruna) GetUniversalFS() (*runtime.UniversalFS, error) {
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
	return &Kiruna{
		Config: config,
	}
}

type WatchedFile = common.WatchedFile
type WatchedFiles = common.WatchedFiles
type OnChangeFunc = common.OnChangeFunc
type OnChange = common.OnChange
type IgnorePatterns = common.IgnorePatterns

const OnChangeStrategyConcurrent = common.OnChangeStrategyConcurrent
const OnChangeStrategyPost = common.OnChangeStrategyPost
const OnChangeStrategyPre = common.OnChangeStrategyPre

var SetupDistDir = buildtime.SetupDistDir
var GetFreePort = util.GetFreePort
var GetIsDev = common.KirunaEnv.GetIsDev
