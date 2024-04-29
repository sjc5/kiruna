package runtime

import (
	"net/http"

	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/util"
)

func GetServeStaticHandler(config *common.Config, pathPrefix string, cacheImmutably bool) http.Handler {
	FS, err := GetPublicFS(config)
	if err != nil {
		util.Log.Errorf("error getting public FS: %v", err)
	}
	if cacheImmutably {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			http.StripPrefix(pathPrefix, http.FileServer(http.FS(FS))).ServeHTTP(w, r)
		})
	}
	return http.StripPrefix(pathPrefix, http.FileServer(http.FS(FS)))
}
