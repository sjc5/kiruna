package dev

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/sjc5/kiruna/internal/buildtime"
	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/runtime"
	"github.com/sjc5/kiruna/internal/util"
)

const defaultFreePort = 10_000
const healthCheckWarningA = `WARNING: No healthcheck endpoint found, setting to "/".`
const healthCheckWarningB = `To set this explicitly, use the "HealthcheckEndpoint" field in your dev config.`
const healthCheckWarning = healthCheckWarningA + "\n" + healthCheckWarningB

func MustStartDev(config *common.Config) {
	// Short circuit if no dev config
	if config.DevConfig == nil {
		errMsg := "error: no dev config found"
		util.Log.Error(errMsg)
		panic(errMsg)
	}

	if len(config.DevConfig.HealthcheckEndpoint) == 0 {
		util.Log.Warning(healthCheckWarning)
		config.DevConfig.HealthcheckEndpoint = "/"
	}

	common.KirunaEnv.SetModeToDev()

	// Warm port right away, in case default is unavailable
	// Also, env needs to be set in this scope
	util.MustGetPort()

	// Set refresh server port
	if freePort, err := util.GetFreePort(defaultFreePort); err == nil {
		common.KirunaEnv.SetRefreshServerPort(freePort)
	} else {
		util.Log.Errorf("error: failed to get free port for refresh server: %v", err)
		panic(err)
	}

	err := buildtime.Build(config, false)
	if err != nil {
		errMsg := fmt.Sprintf("error: failed to build app: %v", err)
		util.Log.Error(errMsg)
		panic(errMsg)
	}

	if config.DevConfig.ServerOnly {
		mustSetupWatcher(nil, config)
		return
	}

	util.Log.Infof("initializing sidecar refresh server on port %d", common.KirunaEnv.GetRefreshServerPort())

	manager := NewClientManager()
	go manager.start()
	go mustSetupWatcher(manager, config)

	mux := http.NewServeMux()

	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		sseHandler(manager)(w, r)
	})

	mux.HandleFunc("/get-refresh-script-inner", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "text/javascript")
		w.Write([]byte(runtime.GetRefreshScriptInner(common.KirunaEnv.GetRefreshServerPort())))
	})

	if err := http.ListenAndServe(":"+strconv.Itoa(common.KirunaEnv.GetRefreshServerPort()), mux); err != nil {
		errMsg := fmt.Sprintf("error: failed to start refresh server: %v", err)
		util.Log.Error(errMsg)
		panic(errMsg)
	}
}
