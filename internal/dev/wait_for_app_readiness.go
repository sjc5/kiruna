package dev

import (
	"net/http"
	"time"

	"github.com/sjc5/kiruna/internal/common"
)

func waitForAppReadiness(config *common.Config) bool {
	maxAttempts := config.DevConfig.MaxReadinessAttempts
	if maxAttempts == 0 {
		maxAttempts = 30
	}
	readinessSleepTime := config.DevConfig.ReadinessSleepTime
	if readinessSleepTime == 0 {
		readinessSleepTime = 300 * time.Millisecond
	}
	for attempts := 0; attempts < maxAttempts; attempts++ {
		resp, err := http.Get(config.DevConfig.HealthcheckURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			return true
		}
		time.Sleep(readinessSleepTime)
	}
	return false
}
