package common

import (
	"io/fs"
	"path/filepath"
	"time"
)

const (
	PublicFileMapGobName = "public_file_map.gob"
	CSSNormalDirName     = "normal"
	CSSCriticalDirName   = "critical"
)

type Map map[string]string
type Callback func() error
type Extensions map[string]Callback

/*
NOTE: Some of the comments in this file may be outdated.
Ultimately they need to be brought up to date and moved
into the README instead.
*/

type Config struct {
	// If not nil, the embedded file system will be used in production builds
	// If nil, the disk file system will be used in production builds
	// Only relevant in prod (in dev mode, the real disk FS is always used)
	DistFS fs.FS

	/*
		RootDir is the parent directory of the Kiruna-specific directories
		(e.g., "dist", "static" and "styles") as well as the file that
		defines your DistFS variable. It may be (and probably is) the same
		as your project's root, but it doesn't have to be. RootDir should
		be set relative your project's root (where you run your dev and
		build commands from). For example, if your project's root is also
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

	BinOutputFilename string
}

func (c *Config) GetIsUsingEmbeddedFS() bool {
	return c.DistFS != nil
}
func (c *Config) GetCleanRootDir() string {
	return filepath.Clean(c.RootDir)
}

type DevConfig struct {
	// REQUIRED
	HealthcheckURL    string
	RefreshServerPort int

	// OPTIONAL
	MaxReadinessAttempts int
	ReadinessSleepTime   time.Duration

	WatchedFiles WatchedFiles

	// Directories to ignore when watching for changes
	// Should be set relative to the RootDir
	// Default ignored list is "dist" relative to RootDir,
	// and "node_modules" and ".git" relative to the
	// directory from where you run your dev/build commands.
	// IgnoreDirs will be appended to the default list
	IgnoreDirs []string

	ServerOnly bool
}

type WatchedFile struct {
	// OnChange runs before any Kiruna processing, except that as long as "SkipRebuildingNotification"
	// is false (default), Kiruna will send a signal to the browser to show the
	// "Rebuilding..." status message first.
	OnChange func(string) error

	// Use this if you need the binary recompiled before the browser is reloaded
	RecompileBinary bool

	// Use this if your onChange saves a file that will trigger another reload process
	// Or for any other reason if you need it
	RunOnChangeOnly bool

	// Use this if you are using RunOnChangeOnly, but your onchange won't actually
	// trigger another reload process (so you dont get stuck with "Rebuilding..."
	// showing in the browser)
	SkipRebuildingNotification bool

	// Set this to true if you're eagerly evaluating and caching your templates
	// during development. This will trigger a hard restart of the server so your
	// templates re-evaluate, but still can skip recompiling the binary. If you are
	// lazily evaluating your templates during development (i.e., re-reading them on
	// every request), you can leave this as false (the default).
	RestartApp bool
}

type WatchedFiles map[string]WatchedFile

// Two plausible WatchedFiles examples:

// 1. Changing Tailwind globals.css, which should be exported into styles/normal/from-tailwind.css (or whatever)
// We would want this to NOT recompile Go, but only to have Tailwind's output cause Kiruna to hot refresh,
// because only CSS changed. So we want (1) to send a rebuilding signal to the browser, and (2) run the OnChange
// (which re-runs Tailwind), and that's it. To do this, we would set an entry in WatchedFiles to ".css" with
// RunOnChangeOnly: true. This would trigger another round the WatchedFiles checks, because it will have seen
// a change to where Tailwind output the new CSS from the OnChange (e.g., styles/normal/from-tailwind.css),
// and that second round will handle the appropriate refreshing.

// 2. Changing a template file that should re-run Tailwind, but not recompile Go. We would want this to (1) send
// a rebuilding signal to browser, (2) run the OnChange (which re-runs Tailwind), and (3) hard refresh the browser.
// To do this, we would set an entry in WatchedFiles with whatever extension the template file has, and set the
// OnChange to a function that re-runs Tailwind. The other settings will default to false, so this is all we need
// to get this behavior.

// NOTE! If you are not lazily evaluating your templates during development, you will need to set RestartApp
// to true. This is because the default behavior is to only restart the app when a Go file changes, and if
// you are eagerly evaluating and caching your templates, you won't see changes you made to your template when
// you reload the page, because the app won't have restarted. If you are lazily evaluating your templates, you
// can leave this as false (the default).
