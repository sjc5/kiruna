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

const portKey = "PORT"

func (k kirunaEnvType) SetPort(port int) {
	os.Setenv(portKey, fmt.Sprintf("%d", port))
}

func (k kirunaEnvType) GetPort() int {
	port, err := strconv.Atoi(os.Getenv(portKey))
	if err != nil {
		return 0
	}
	return port
}

const portHasBeenSetKey = "KIRUNA_ENV_PORT_HAS_BEEN_SET"

func (k kirunaEnvType) SetPortHasBeenSet() {
	os.Setenv(portHasBeenSetKey, "true")
}

func (k kirunaEnvType) GetPortHasBeenSet() bool {
	return os.Getenv(portHasBeenSetKey) == "true"
}
