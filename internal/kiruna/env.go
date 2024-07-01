package ik

import (
	"fmt"
	"os"
	"strconv"
)

var KirunaEnv = kirunaEnvType{}

const (
	modeKey              = "KIRUNA_ENV_MODE"
	devModeVal           = "development"
	portKey              = "PORT"
	portHasBeenSetKey    = "KIRUNA_ENV_PORT_HAS_BEEN_SET"
	refreshServerPortKey = "KIRUNA_ENV_REFRESH_SERVER_PORT"
	trueStr              = "true"
)

type kirunaEnvType struct{}

func (k kirunaEnvType) GetIsDev() bool {
	return os.Getenv(modeKey) == devModeVal
}

func (k kirunaEnvType) setPort(port int) {
	os.Setenv(portKey, fmt.Sprintf("%d", port))
}

func (k kirunaEnvType) getPort() int {
	port, err := strconv.Atoi(os.Getenv(portKey))
	if err != nil {
		return 0
	}
	return port
}

func (k kirunaEnvType) setPortHasBeenSet() {
	os.Setenv(portHasBeenSetKey, trueStr)
}

func (k kirunaEnvType) getPortHasBeenSet() bool {
	return os.Getenv(portHasBeenSetKey) == trueStr
}

func (k kirunaEnvType) getRefreshServerPort() int {
	port, err := strconv.Atoi(os.Getenv(refreshServerPortKey))
	if err != nil {
		return 0
	}
	return port
}

func (k kirunaEnvType) setModeToDev() {
	os.Setenv(modeKey, devModeVal)
}

func (k kirunaEnvType) setRefreshServerPort(port int) {
	os.Setenv(refreshServerPortKey, fmt.Sprintf("%d", port))
}
