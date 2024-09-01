package ik

import (
	"fmt"
	"os"
	"strconv"
)

const (
	modeKey              = "KIRUNA_ENV_MODE"
	devModeVal           = "development"
	portKey              = "PORT"
	portHasBeenSetKey    = "KIRUNA_ENV_PORT_HAS_BEEN_SET"
	refreshServerPortKey = "KIRUNA_ENV_REFRESH_SERVER_PORT"
	trueStr              = "true"
	isBuildTimeKey       = "KIRUNA_ENV_IS_BUILD_TIME"
)

func getIsDev() bool {
	return os.Getenv(modeKey) == devModeVal
}

func setPort(port int) {
	os.Setenv(portKey, fmt.Sprintf("%d", port))
}

func getPort() int {
	port, err := strconv.Atoi(os.Getenv(portKey))
	if err != nil {
		return 0
	}
	return port
}

func setPortHasBeenSet() {
	os.Setenv(portHasBeenSetKey, trueStr)
}

func getPortHasBeenSet() bool {
	return os.Getenv(portHasBeenSetKey) == trueStr
}

func getRefreshServerPort() int {
	port, err := strconv.Atoi(os.Getenv(refreshServerPortKey))
	if err != nil {
		return 0
	}
	return port
}

func setModeToDev() {
	os.Setenv(modeKey, devModeVal)
}

func setRefreshServerPort(port int) {
	os.Setenv(refreshServerPortKey, fmt.Sprintf("%d", port))
}

func setIsBuildTime(val bool) {
	if val {
		os.Setenv(isBuildTimeKey, trueStr)
	} else {
		os.Setenv(isBuildTimeKey, "")
	}
}

func getIsBuildTime() bool {
	return os.Getenv(isBuildTimeKey) == trueStr
}
