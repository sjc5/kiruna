package ik

import (
	"html/template"
	"sync"

	"github.com/sjc5/kit/pkg/typed"
)

var cache = struct {
	// FS
	uniFS    typed.SyncMap[*Config, UniversalFS]
	uniDirFS typed.SyncMap[*Config, UniversalFS]
	fsType   typed.SyncMap[*Config, string]

	// CSS
	styleSheetElement typed.SyncMap[*Config, template.HTML]
	criticalCSS       typed.SyncMap[*Config, *criticalCSSStatus]
	styleSheetURL     typed.SyncMap[*Config, string]

	// Public URLs
	fileMapFromGob   typed.SyncMap[string, map[string]string]
	fileMapLoadOnce  typed.SyncMap[string, *sync.Once]
	publicURLs       typed.SyncMap[string, string]
	publicURLsMap    typed.SyncMap[string, map[string]string]
	publicFileMapURL typed.SyncMap[*Config, string]

	// Dev
	matchResults typed.SyncMap[string, bool]
}{
	// FS
	uniFS:    typed.SyncMap[*Config, UniversalFS]{},
	uniDirFS: typed.SyncMap[*Config, UniversalFS]{},
	fsType:   typed.SyncMap[*Config, string]{},

	// CSS
	styleSheetElement: typed.SyncMap[*Config, template.HTML]{},
	criticalCSS:       typed.SyncMap[*Config, *criticalCSSStatus]{},
	styleSheetURL:     typed.SyncMap[*Config, string]{},

	// Public URLs
	fileMapFromGob:  typed.SyncMap[string, map[string]string]{},
	fileMapLoadOnce: typed.SyncMap[string, *sync.Once]{},
	publicURLs:      typed.SyncMap[string, string]{},

	// Dev
	matchResults: typed.SyncMap[string, bool]{},
}
