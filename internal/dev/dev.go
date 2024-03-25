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

func Dev(config *common.Config) {
	common.SetKirunaEnvDev()

	var err error
	if config.DevConfig.RefreshServerPort == 0 {
		config.DevConfig.RefreshServerPort, err = util.GetFreePort()
		if err != nil {
			fmt.Printf("error: failed to get free port: %v\n", err)
			fmt.Printf("using default port number %d\n", 51027)
			fmt.Printf("to specify a different port for the sidecar dev refresh server, manually set DevConfig.RefreshServerPort in your config\n")
			config.DevConfig.RefreshServerPort = 51027
			return
		}
	}

	if config.DevConfig == nil {
		util.Log.Panicf("error: no dev config found")
		return
	}

	err = buildtime.Build(config, false)
	if err != nil {
		util.Log.Panicf("error: build process failed: %v", err)
	}

	if config.DevConfig.ServerOnly {
		setupWatcher(nil, config)
	} else {
		util.Log.Infof("initializing sidecar refresh server on port number %d", config.DevConfig.RefreshServerPort)

		manager := NewClientManager()
		go manager.start()
		go setupWatcher(manager, config)

		mux := http.NewServeMux()

		mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			sseHandler(manager)(w, r)
		})

		mux.HandleFunc("/get-refresh-script-inner", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Content-Type", "text/javascript")
			w.Write([]byte(runtime.GetRefreshScriptInner(config.DevConfig.RefreshServerPort)))
		})

		if err := http.ListenAndServe(":"+strconv.Itoa(config.DevConfig.RefreshServerPort), mux); err != nil {
			util.Log.Panicf("error: failed to start refresh server: %v", err)
		}
	}
}
