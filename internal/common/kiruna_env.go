package common

import (
	"fmt"
	"os"
	"strconv"
)

type kirunaEnvType struct{}

var KirunaEnv = kirunaEnvType{}

const modeKey = "KIRUNA_ENV_MODE"
const devModeVal = "development"

func (k kirunaEnvType) SetModeToDev() {
	os.Setenv(modeKey, devModeVal)
}
func (k kirunaEnvType) GetIsDev() bool {
	return os.Getenv(modeKey) == devModeVal
}

const refreshServerPortKey = "KIRUNA_ENV_REFRESH_SERVER_PORT"

func (k kirunaEnvType) SetRefreshServerPort(port int) {
	os.Setenv(refreshServerPortKey, fmt.Sprintf("%d", port))
}

func (k kirunaEnvType) GetRefreshServerPort() int {
	port, err := strconv.Atoi(os.Getenv(refreshServerPortKey))
	if err != nil {
		return 0
	}
	return port
}
