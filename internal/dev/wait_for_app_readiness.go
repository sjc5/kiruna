package dev

import (
	"net/http"
	"time"

	"github.com/sjc5/kiruna/internal/common"
)

func waitForAppReadiness(config *common.Config) bool {
	maxAttempts := config.DevConfig.MaxReadinessAttempts
	if maxAttempts == 0 {
		maxAttempts = 100
	}
	readinessSleepTime := config.DevConfig.ReadinessSleepTime
	if readinessSleepTime == 0 {
		readinessSleepTime = 20 * time.Millisecond
	}
	for attempts := 0; attempts < maxAttempts; attempts++ {
		resp, err := http.Get(config.DevConfig.HealthcheckURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			return true
		}
		additionalDelay := time.Duration(attempts * 20)
		time.Sleep(readinessSleepTime + additionalDelay*time.Millisecond)
	}
	return false
}
