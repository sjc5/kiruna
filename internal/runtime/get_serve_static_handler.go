package runtime

import (
	"net/http"

	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/util"
)

func GetServeStaticHandler(config *common.Config, pathPrefix string) http.Handler {
	FS, err := GetPublicFS(config)
	if err != nil {
		util.Log.Panicf("error getting public FS: %v", err)
	}
	return http.StripPrefix(pathPrefix, http.FileServer(http.FS(FS)))
}
