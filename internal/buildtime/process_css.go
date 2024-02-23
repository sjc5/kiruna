package buildtime

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/runtime"
	"github.com/sjc5/kiruna/internal/util"
)

var UrlRegex = regexp.MustCompile(`url\(([^)]+)\)`)

// ProcessCSS concatenates and hashes specified CSS files, then saves them to disk.
func ProcessCSS(config *common.Config, subDir string) error {
	cleanRootDir := config.GetCleanRootDir()

	dirPath := filepath.Join(cleanRootDir, "styles", subDir)
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return nil
	}
	files, err := os.ReadDir(dirPath)
	if err != nil {
		util.Log.Errorf("error reading directory: %v", err)
		return err
	}

	var concatenatedCSS strings.Builder
	var fileNames []string

	// Collect and sort .css files
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".css") {
			fileNames = append(fileNames, file.Name())
		}
	}
	sort.Strings(fileNames)

	// Concatenate file contents
	for _, fileName := range fileNames {
		content, err := os.ReadFile(filepath.Join(dirPath, fileName))
		if err != nil {
			util.Log.Errorf("error reading file: %v", err)
			return err
		}
		concatenatedCSS.Write(content)
	}

	concatenatedCSSString := concatenatedCSS.String()
	concatenatedCSSString = UrlRegex.ReplaceAllStringFunc(concatenatedCSSString, func(match string) string {
		rawUrl := UrlRegex.FindStringSubmatch(match)[1]
		cleanedUrl := strings.TrimSpace(strings.Trim(rawUrl, "'\""))
		if !strings.HasPrefix(cleanedUrl, "http") && !strings.Contains(cleanedUrl, "://") {
			hashedUrl := runtime.GetPublicURL(config, cleanedUrl)
			return fmt.Sprintf("url(%s)", hashedUrl)
		} else {
			return match // Leave external URLs unchanged
		}
	})

	// Determine output path and filename
	var outputPath string

	switch subDir {
	case "critical":
		outputPath = filepath.Join(cleanRootDir, "dist", "kiruna", "internal")
	case "normal":
		outputPath = filepath.Join(cleanRootDir, "dist", "kiruna", "static", "public", "hashed")
	}

	outputFileName := subDir + ".css" // Default for 'critical'

	if subDir == "normal" {
		// first, delete the old normal.css file(s)
		oldNormalPath := filepath.Join(outputPath, "normal_*.css")
		oldNormalFiles, err := filepath.Glob(oldNormalPath)
		if err != nil {
			util.Log.Errorf("error finding old normal CSS files: %v", err)
			return err
		}
		for _, oldNormalFile := range oldNormalFiles {
			if err := os.Remove(oldNormalFile); err != nil {
				util.Log.Errorf("error removing old normal CSS file: %v", err)
				return err
			}
		}

		// Hash the concatenated content
		outputFileName = util.GetHashedFilename(
			[]byte(concatenatedCSS.String()),
			"normal.css",
		)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		util.Log.Errorf("error creating output directory: %v", err)
		return err
	}

	// Write concatenated content to file
	outputFile := filepath.Join(outputPath, outputFileName)

	// If normal, also write to a file called normal_css_ref.txt with the hash
	if subDir == "normal" {
		hashFile := filepath.Join(cleanRootDir, "dist", "kiruna", "internal", "normal_css_file_ref.txt")
		if err := os.WriteFile(hashFile, []byte(outputFileName), 0644); err != nil {
			util.Log.Errorf("error writing to file: %v", err)
			return err
		}
	}

	if subDir == "critical" {
		concatenatedCSSString = naiveCSSMinify(concatenatedCSS.String())
	}

	if err := os.WriteFile(outputFile, []byte(concatenatedCSSString), 0644); err != nil {
		util.Log.Errorf("error writing to file: %v", err)
		return err
	}

	return nil
}

func naiveCSSMinify(content string) string {
	return strings.Join(strings.Fields(content), " ")
}
