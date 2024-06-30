package ik

import (
	"io/fs"

	"github.com/sjc5/kit/pkg/colorlog"
)

type Logger colorlog.Logger

type Config struct {
	/*
		If not nil, the embedded file system will be used in production builds.
		If nil, the disk file system will be used in production builds.
		Only relevant in prod (in dev mode, the real disk FS is always used).
		If nil in prod, you need to make sure that you ship the dist directory
		with your binary. For simplicity, we recommend using the embedded FS.
	*/
	DistFS fs.FS

	/*
		RootDir is the parent directory of the Kiruna-specific directories
		(e.g., "dist", "static" and "styles"). It may be (and probably is)
		the same as your project's root, but it doesn't have to be. RootDir
		should be set relative your project's root (where you run your dev
		and build commands from). For example, if your project's root is also
		your RootDir, then RootDir should be set to ".". If RootDir is a
		subdirectory of your project's root, then set it accordingly (e.g.,
		"./app" or "./kiruna"). We do run filepath.Clean on the RootDir,
		so if you leave it blank, it will default to ".".
	*/
	RootDir string

	// Set EntryPoint relative to the RootDir, e.g., "./cmd/app/main.go".
	// Note that your RootDir may be the same as your project's root, but
	// it isn't necessarily so. See the RootDir comment for more info.
	EntryPoint string

	DevConfig *DevConfig

	Logger Logger
}

type DevConfig struct {
	HealthcheckEndpoint string // e.g., "/healthz" -- should return 200 OK if healthy -- defaults to "/"
	WatchedFiles        WatchedFiles
	IgnorePatterns      IgnorePatterns
	ServerOnly          bool
}

type WatchedFile struct {
	Pattern string // Glob pattern (set relative to Config.RootDir)

	// By default, OnChange runs before any Kiruna processing. As long as "SkipRebuildingNotification"
	// is false (default), Kiruna will send a signal to the browser to show the
	// "Rebuilding..." status message first. You can also change the OnChange strategy to
	// "post" or "concurrent" if desired.
	OnChangeCallbacks []OnChange

	// Use this if your onChange saves a file that will trigger another reload process,
	// or if you need this behavior for any other reason. Will not reload the browser.
	// Note that if you use this setting, you should not set an explicit strategy on
	// the OnChange callbacks (or set them explicitly to "pre"). If you set them to
	// "post" or "concurrent" while using RunOnChangeOnly, the OnChange callbacks will
	// not run.
	RunOnChangeOnly bool

	// Use this if you are using RunOnChangeOnly, but your onchange won't actually
	// trigger another reload process (so you dont get stuck with "Rebuilding..."
	// showing in the browser)
	SkipRebuildingNotification bool

	// Use this if you need the binary recompiled before the browser is reloaded
	RecompileBinary bool

	// Use this if you explicitly need the app to be restarted before reloading the browser.
	// Example: You might need this if you memory cache template files on first hit, in which
	// case you would want to restart the app to clear the cache.
	RestartApp bool

	// This may come into play if you have a .go file that is totally independent from you
	// app, such as a wasm file that you are building with a separate build process and serving
	// from your app. If you set this to true, processing on any captured .go file will be as
	// though it were an arbitrary non-Go file extension. Only relevant for Go files -- for
	// non-Go files, this is a no-op.
	TreatAsNonGo bool
}

type OnChangeFunc func(string) error

type OnChange struct {
	Strategy         string
	Func             OnChangeFunc
	ExcludedPatterns []string // Glob patterns (set relative to Config.RootDir)
}

type WatchedFiles []WatchedFile

type IgnorePatterns struct {
	Dirs  []string // Glob patterns
	Files []string // Glob patterns
}
