package util

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

const maxOffset = 100

func GetFreePort(defaultPort int) (int, error) {
	if port, err := checkPortAvailability(defaultPort); err == nil {
		return port, nil
	}

	for offset := 1; offset <= maxOffset; offset++ {
		for _, port := range []int{defaultPort + offset, defaultPort} {
			if port >= 0 && port <= 65535 {
				if port, err := checkPortAvailability(port); err == nil {
					return port, nil
				}
			}
		}
	}

	return getRandomFreePort()
}

func checkPortAvailability(port int) (int, error) {
	fakeServer := &http.Server{Addr: fmt.Sprintf(":%d", port)}
	defer fakeServer.Close()

	go func() {
		time.Sleep(1 * time.Millisecond)
		fakeServer.Close()
	}()

	err := fakeServer.ListenAndServe()
	if err != http.ErrServerClosed {
		fmt.Printf("port %d is unavailable\n", port)
		return 0, err
	}

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
