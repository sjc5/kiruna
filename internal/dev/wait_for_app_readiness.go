package dev

import (
	"fmt"
	"net/http"
	"time"

	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/util"
)

const maxAttempts = 100
const readinessSleepTime = 20 * time.Millisecond

func waitForAppReadiness(config *common.Config) bool {
	for attempts := 0; attempts < maxAttempts; attempts++ {
		url := fmt.Sprintf(
			"http://localhost:%d%s",
			util.MustGetPort(),
			config.DevConfig.HealthcheckEndpoint,
		)

		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			return true
		}

		additionalDelay := time.Duration(attempts * 20)
		time.Sleep(readinessSleepTime + additionalDelay*time.Millisecond)
	}
	return false
}
