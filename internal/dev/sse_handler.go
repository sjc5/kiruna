package dev

import (
	"fmt"
	"net/http"
	"time"
)

func sseHandler(manager *ClientManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		msg := make(chan RefreshFilePayload, 1)
		client := &Client{id: r.RemoteAddr, notify: msg}
		manager.register <- client

		defer func() {
			manager.unregister <- client
		}()

		go func() {
			<-r.Context().Done()
			manager.unregister <- client
		}()

		for {
			select {
			case m := <-msg:
				// encode as json
				json := fmt.Sprintf(
					`{"changeType": "%s", "criticalCss": "%s", "normalCssUrl": "%s", "at": "%s"}`,
					m.ChangeType,
					m.CriticalCSS,
					m.NormalCSSURL,
					m.At.Format(time.RFC3339),
				)
				fmt.Fprintf(w, "data: %s\n\n", json)
				w.(http.Flusher).Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}
