package ik

import (
	"os"
	"testing"
)

func TestGetIsDev(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     bool
	}{
		{"DevMode", devModeVal, true},
		{"ProdMode", "production", false},
		{"EmptyMode", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetEnv()
			if tt.envValue != "" {
				os.Setenv(modeKey, tt.envValue)
			}
			if got := getIsDev(); got != tt.want {
				t.Errorf("getIsDev() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPortFunctions(t *testing.T) {
	resetEnv()

	// Test setPort and getPort
	setPort(8080)
	if got := getPort(); got != 8080 {
		t.Errorf("getPort() = %v, want %v", got, 8080)
	}

	// Test setPortHasBeenSet and getPortHasBeenSet
	if getPortHasBeenSet() {
		t.Errorf("getPortHasBeenSet() = true, want false before setting")
	}
	setPortHasBeenSet()
	if !getPortHasBeenSet() {
		t.Errorf("getPortHasBeenSet() = false, want true after setting")
	}
}

func TestRefreshServerPort(t *testing.T) {
	resetEnv()

	setRefreshServerPort(3000)
	if got := getRefreshServerPort(); got != 3000 {
		t.Errorf("getRefreshServerPort() = %v, want %v", got, 3000)
	}
}

func TestBuildTimeFunctions(t *testing.T) {
	resetEnv()

	if getIsBuildTime() {
		t.Errorf("getIsBuildTime() = true, want false before setting")
	}

	setIsBuildTime(true)
	if !getIsBuildTime() {
		t.Errorf("getIsBuildTime() = false, want true after setting")
	}

	setIsBuildTime(false)
	if getIsBuildTime() {
		t.Errorf("getIsBuildTime() = true, want false after unsetting")
	}
}
