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

func MustStartDev(config *common.Config) {
	common.KirunaEnv.SetModeToDev()

	if config.DevConfig.RefreshServerPort == 0 {
		freePort, err := util.GetFreePort()
		if err != nil {
			freePort = 51027 // just a "random" port that is likely to be free
			fmt.Printf("error: failed to get free port: %v\n", err)
			fmt.Printf("attempting to use default fallback port: %d\n", freePort)
			fmt.Printf("to specify a different port for the sidecar dev refresh server, manually set DevConfig.RefreshServerPort in your config\n")
		} else {
			common.KirunaEnv.SetRefreshServerPort(freePort)
		}
	} else {
		common.KirunaEnv.SetRefreshServerPort(config.DevConfig.RefreshServerPort)
	}

	if config.DevConfig == nil {
		util.Log.Panicf("error: no dev config found")
		return
	}

	err := buildtime.Build(config, false)
	if err != nil {
		util.Log.Panicf("error: failed to build app: %v", err)
		return
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
			util.Log.Panicf("error: failed to start refresh server: %v", err)
		}
	}
}
