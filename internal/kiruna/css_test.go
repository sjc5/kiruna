package ik

import (
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetCriticalCSS(t *testing.T) {
	env := setupTestEnv(t)
	defer teardownTestEnv(t)

	criticalCSS := "body { color: red; }"
	env.createTestFile(t, "dist/kiruna/internal/critical.css", criticalCSS)

	result := env.config.GetCriticalCSS()
	if result != criticalCSS {
		t.Errorf("GetCriticalCSS() = %v, want %v", result, criticalCSS)
	}
}

func TestGetCriticalCSSStyleElement(t *testing.T) {
	env := setupTestEnv(t)
	defer teardownTestEnv(t)

	criticalCSS := "body { color: red; }"
	env.createTestFile(t, "dist/kiruna/internal/critical.css", criticalCSS)

	result := env.config.GetCriticalCSSStyleElement()
	expected := template.HTML(`<style id="` + CriticalCSSElementID + `">body { color: red; }</style>`)
	if result != expected {
		t.Errorf("GetCriticalCSSStyleElement() = %v, want %v", result, expected)
	}
}

func TestGetStyleSheetURL(t *testing.T) {
	env := setupTestEnv(t)
	defer teardownTestEnv(t)

	normalCSSFile := "normal_1234567890.css"
	env.createTestFile(t, "dist/kiruna/internal/normal_css_file_ref.txt", normalCSSFile)

	result := env.config.GetStyleSheetURL()
	expected := "/public/" + normalCSSFile
	if result != expected {
		t.Errorf("GetStyleSheetURL() = %v, want %v", result, expected)
	}
}

func TestGetStyleSheetLinkElement(t *testing.T) {
	env := setupTestEnv(t)
	defer teardownTestEnv(t)

	normalCSSFile := "normal_1234567890.css"
	env.createTestFile(t, "dist/kiruna/internal/normal_css_file_ref.txt", normalCSSFile)

	result := env.config.GetStyleSheetLinkElement()
	expected := template.HTML(`<link rel="stylesheet" href="/public/` + normalCSSFile + `" id="` + StyleSheetElementID + `" />`)
	if result != expected {
		t.Errorf("GetStyleSheetLinkElement() = %v, want %v", result, expected)
	}
}

func TestBuildCSS(t *testing.T) {
	env := setupTestEnv(t)
	defer teardownTestEnv(t)

	// Create test CSS files in the source directory
	criticalCSS := "body { color: red; }"
	normalCSS := "p { font-size: 16px; }"
	env.createTestFile(t, "styles/critical/main.css", criticalCSS)
	env.createTestFile(t, "styles/normal/main.css", normalCSS)

	minimizedCriticalCSS := "body{color:red}"
	minimizedNormalCSS := "p{font-size:16px}"

	err := env.config.buildCSS()
	if err != nil {
		t.Fatalf("buildCSS() error = %v", err)
	}

	// Check if critical CSS was processed correctly
	processedCriticalCSS, err := os.ReadFile(filepath.Join(testRootDir, "dist/kiruna/internal/critical.css"))
	if err != nil {
		t.Fatalf("Failed to read processed critical CSS: %v", err)
	}
	if string(processedCriticalCSS) != minimizedCriticalCSS {
		t.Errorf("Processed critical CSS = %v, want %v", string(processedCriticalCSS), criticalCSS)
	}

	// Check if normal CSS reference file was created and points to an existing file
	normalCSSRef, err := os.ReadFile(filepath.Join(testRootDir, "dist/kiruna/internal/normal_css_file_ref.txt"))
	if err != nil {
		t.Fatalf("Failed to read normal CSS reference file: %v", err)
	}
	normalCSSFilename := strings.TrimSpace(string(normalCSSRef))
	if !strings.HasPrefix(normalCSSFilename, "normal_") || !strings.HasSuffix(normalCSSFilename, ".css") {
		t.Errorf("Invalid normal CSS reference: %v", normalCSSFilename)
	}

	// Check if the referenced normal CSS file exists and has correct content
	normalCSSPath := filepath.Join(testRootDir, "dist/kiruna/static/public", normalCSSFilename)
	if _, err := os.Stat(normalCSSPath); os.IsNotExist(err) {
		t.Fatalf("Referenced normal CSS file does not exist: %s", normalCSSPath)
	}

	processedNormalCSS, err := os.ReadFile(normalCSSPath)
	if err != nil {
		t.Fatalf("Failed to read processed normal CSS: %v", err)
	}
	if string(processedNormalCSS) != minimizedNormalCSS {
		t.Errorf("Processed normal CSS = %v, want %v", string(processedNormalCSS), normalCSS)
	}
}