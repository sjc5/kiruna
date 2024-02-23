package dev

import (
	"net/http"
	"strconv"

	"github.com/sjc5/kiruna/internal/buildtime"
	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/util"
)

func Dev(config *common.Config) {
	common.SetKirunaEnvDev()

	if config.DevConfig == nil {
		util.Log.Panicf("error: no dev config found")
		return
	}

	err := buildtime.Build(config, false)
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

		if err := http.ListenAndServe(":"+strconv.Itoa(config.DevConfig.RefreshServerPort), mux); err != nil {
			util.Log.Panicf("error: failed to start refresh server: %v", err)
		}
	}
}
