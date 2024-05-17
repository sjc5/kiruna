package util

import (
	"fmt"
	"log"
	"net"

	"github.com/sjc5/kiruna/internal/common"
)

const maxOffset = 1024

func MustGetPort() int {
	isDev := common.KirunaEnv.GetIsDev()
	portHasBeenSet := common.KirunaEnv.GetPortHasBeenSet()
	defaultPort := common.KirunaEnv.GetPort()

	if !isDev || portHasBeenSet {
		return defaultPort
	}

	port, err := GetFreePort(defaultPort)
	if err != nil {
		log.Panicf("error: failed to get free port: %v", err)
	}

	common.KirunaEnv.SetPort(port)
	common.KirunaEnv.SetPortHasBeenSet()

	return port
}

func GetFreePort(defaultPort int) (int, error) {
	if defaultPort == 0 {
		defaultPort = 8080
	}

	if port, err := checkPortAvailability(defaultPort); err == nil {
		return port, nil
	}

	for i := range maxOffset {
		port := defaultPort + i
		if port >= 0 && port <= 65535 {
			if port, err := checkPortAvailability(port); err == nil {
				Log.Warningf(
					"port %d unavailable: falling back to port %d",
					defaultPort,
					port,
				)
				return port, nil
			}
		} else {
			break
		}
	}

	port, err := getRandomFreePort()
	if err != nil {
		return defaultPort, err
	}

	return port, nil
}

func checkPortAvailability(port int) (int, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return 0, err
	}
	defer ln.Close()

	return port, nil
}

func getRandomFreePort() (port int, err error) {
	// Asks the kernel for a free open port that is ready to use.
	// Credit: https://gist.github.com/sevkin/96bdae9274465b2d09191384f86ef39d
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			return l.Addr().(*net.TCPAddr).Port, nil
		}
	}
	return
}
