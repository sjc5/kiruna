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

const defaultFreePort = 51027

func MustStartDev(config *common.Config) {
	common.KirunaEnv.SetModeToDev()

	// Warm port right away, in case default is unavailable
	// Also, env needs to be set in this scope
	util.MustGetPort()

	if config.DevConfig.RefreshServerPort == 0 {
		freePort, err := util.GetFreePort(defaultFreePort)
		if err != nil {
			util.Log.Errorf("error: failed to get free port: %v", err)
			panic(err)
		} else {
			common.KirunaEnv.SetRefreshServerPort(freePort)
		}
	} else {
		common.KirunaEnv.SetRefreshServerPort(config.DevConfig.RefreshServerPort)
	}

	if config.DevConfig == nil {
		errMsg := "error: no dev config found"
		util.Log.Error(errMsg)
		panic(errMsg)
	}

	err := buildtime.Build(config, false)
	if err != nil {
		errMsg := fmt.Sprintf("error: failed to build app: %v", err)
		util.Log.Error(errMsg)
		panic(errMsg)
	}

	if config.DevConfig.ServerOnly {
		mustSetupWatcher(nil, config)
	} else {
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
}
