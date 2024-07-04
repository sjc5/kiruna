package ik

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

type EvtDetails struct {
	evt                 *fsnotify.Event
	isIgnored           bool
	isGo                bool
	isOther             bool
	isCriticalCSS       bool
	isNormalCSS         bool
	isKirunaCSS         bool
	wfc                 *WatchedFile
	isNonEmptyCHMODOnly bool
}

func (c *Config) getEvtDetails(evt fsnotify.Event) *EvtDetails {
	isCssSimple := filepath.Ext(evt.Name) == ".css"
	isCriticalCSS := isCssSimple && c.getIsCssEvtType(evt, changeTypeCriticalCSS)
	isNormalCSS := isCssSimple && c.getIsCssEvtType(evt, changeTypeNormalCSS)
	isKirunaCSS := isCriticalCSS || isNormalCSS

	var matchingWatchedFile *WatchedFile

	for _, wfc := range c.DevConfig.WatchedFiles {
		isMatch := c.getIsMatch(potentialMatch{pattern: wfc.Pattern, path: evt.Name})
		if isMatch {
			matchingWatchedFile = &wfc
			break
		}
	}

	if matchingWatchedFile == nil {
		for _, wfc := range *c.defaultWatchedFiles {
			isMatch := c.getIsMatch(potentialMatch{pattern: wfc.Pattern, path: evt.Name})
			if isMatch {
				matchingWatchedFile = &wfc
				break
			}
		}
	}

	isGo := filepath.Ext(evt.Name) == ".go"
	if isGo && matchingWatchedFile != nil && matchingWatchedFile.TreatAsNonGo {
		isGo = false
	}

	isOther := !isGo && !isKirunaCSS

	isIgnored := c.getIsIgnored(evt.Name, c.ignoredFilePatterns)
	if isOther && matchingWatchedFile == nil {
		isIgnored = true
	}

	return &EvtDetails{
		evt:                 &evt,
		isOther:             isOther,
		isKirunaCSS:         isKirunaCSS,
		isGo:                isGo,
		isIgnored:           isIgnored,
		isCriticalCSS:       isCriticalCSS,
		isNormalCSS:         isNormalCSS,
		wfc:                 matchingWatchedFile,
		isNonEmptyCHMODOnly: c.getIsNonEmptyCHMODOnly(evt),
	}
}

func (c *Config) getIsEmptyFile(evt fsnotify.Event) bool {
	file, err := os.Open(evt.Name)
	if err != nil {
		c.Logger.Errorf("error: failed to open file: %v", err)
		return false
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		c.Logger.Errorf("error: failed to get file stats: %v", err)
		return false
	}
	return stat.Size() == 0
}

func (c *Config) getIsCssEvtType(evt fsnotify.Event, cssType changeType) bool {
	return strings.HasPrefix(evt.Name, filepath.Join(c.getCleanRootDir(), "styles/"+string(cssType)))
}

func (c *Config) getIsNonEmptyCHMODOnly(evt fsnotify.Event) bool {
	isSolelyCHMOD := !evt.Has(fsnotify.Write) && !evt.Has(fsnotify.Create) && !evt.Has(fsnotify.Remove) && !evt.Has(fsnotify.Rename)
	return isSolelyCHMOD && !c.getIsEmptyFile(evt)
}
