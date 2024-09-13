package ik

import (
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/sjc5/kit/pkg/bytesutil"
	"github.com/sjc5/kit/pkg/cryptoutil"
	"github.com/sjc5/kit/pkg/htmlutil"
)

// clientManager manages all SSE clients
type clientManager struct {
	clients    map[*client]bool
	register   chan *client
	unregister chan *client
	broadcast  chan refreshFilePayload
}

// Client represents a single SSE connection
type client struct {
	id     string
	notify chan<- refreshFilePayload
}

type Base64 = string

type refreshFilePayload struct {
	ChangeType   changeType `json:"changeType"`
	CriticalCSS  Base64     `json:"criticalCSS"`
	NormalCSSURL string     `json:"normalCSSURL"`
	At           time.Time  `json:"at"`
}

type changeType string

const (
	changeTypeNormalCSS   changeType = "normal"
	changeTypeCriticalCSS changeType = "critical"
	changeTypeOther       changeType = "other"
	changeTypeRebuilding  changeType = "rebuilding"
	changeTypeRevalidate  changeType = "revalidate"
)

func newClientManager() *clientManager {
	return &clientManager{
		clients:    make(map[*client]bool),
		register:   make(chan *client),
		unregister: make(chan *client),
		broadcast:  make(chan refreshFilePayload),
	}
}

// Start the manager to handle clients and broadcasting
func (manager *clientManager) start() {
	for {
		select {
		case client := <-manager.register:
			manager.clients[client] = true
		case client := <-manager.unregister:
			if _, ok := manager.clients[client]; ok {
				delete(manager.clients, client)
				close(client.notify)
			}
		case msg := <-manager.broadcast:
			for client := range manager.clients {
				client.notify <- msg
			}
		}
	}
}

func (c *Config) mustReloadBroadcast(rfp refreshFilePayload) {
	if c.waitForAppReadiness() {
		c.manager.broadcast <- rfp
		return
	}
	errMsg := fmt.Sprintf("error: app never became ready: %v", rfp.ChangeType)
	c.Logger.Error(errMsg)
	panic(errMsg)
}

func (c *Config) GetRefreshScriptSha256Hash() string {
	if !getIsDev() {
		return ""
	}
	hash := cryptoutil.Sha256Hash([]byte(GetRefreshScriptInner(getRefreshServerPort())))
	return bytesutil.ToBase64(hash)
}

func (c *Config) GetRefreshScript() template.HTML {
	if !getIsDev() {
		return ""
	}
	result, _ := htmlutil.RenderElement(&htmlutil.Element{
		Tag:       "script",
		InnerHTML: template.HTML(GetRefreshScriptInner(getRefreshServerPort())),
	})
	return result
}

func GetRefreshScriptInner(port int) string {
	return fmt.Sprintf(refreshScriptFmt, port)
}

// changeTypes: "rebuilding", "other", "normal", "critical", "revalidate"
// Element IDs: "__refreshscript-rebuilding", "__normal-css", "__critical-css"
const refreshScriptFmt = `
	function base64ToUTF8(base64) {
	const bytes = Uint8Array.from(atob(base64), (m) => m.codePointAt(0) || 0);
	return new TextDecoder().decode(bytes);
	}

	const scrollYKey = "__kiruna_internal__devScrollY";
	const scrollY = localStorage.getItem(scrollYKey);
	if (scrollY) {
		setTimeout(() => {
			localStorage.removeItem(scrollYKey);
			console.info("KIRUNA DEV: Restoring previous scroll position");
			window.scrollTo({ top: scrollY, behavior: "smooth" })
		}, 150);
	}

	const es = new EventSource("http://localhost:%d/events");

	es.onmessage = (e) => {
		const { changeType, criticalCSS, normalCSSURL, at } = JSON.parse(e.data);
		if (changeType == "rebuilding") {
			console.log("KIRUNA DEV: Rebuilding server...");
			const el = document.createElement("div");
			el.innerHTML = "Rebuilding...";
			el.id = "__refreshscript-rebuilding";
			el.style.display = "flex";
			el.style.position = "fixed";
			el.style.inset = "0";
			el.style.width = "100%%";
			el.style.backgroundColor = "#333a";
			el.style.color = "white";
			el.style.textAlign = "center";
			el.style.padding = "10px";
			el.style.zIndex = "1000";
			el.style.fontSize = "7vw";
			el.style.fontWeight = "bold";
			el.style.textShadow = "2px 2px 2px #000";
			el.style.justifyContent = "center";
			el.style.alignItems = "center";
			el.style.opacity = "0";
			el.style.transition = "opacity 0.05s";
			document.body.appendChild(el);
			setTimeout(() => {
				el.style.opacity = "1";
			}, 10);
		}
		if (changeType == "other") {
			const scrollY = window.scrollY;
			if (scrollY > 0) {
				localStorage.setItem(scrollYKey, scrollY);
			}
			window.location.reload();
		}
		if (changeType == "normal") {
			const oldLink = document.getElementById("__normal-css");
			const newLink = document.createElement("link");
			newLink.id = "__normal-css";
			newLink.rel = "stylesheet";
			newLink.href = normalCSSURL;
			newLink.onload = () => oldLink.remove();
			oldLink.parentNode.insertBefore(newLink, oldLink.nextSibling);
		}
		if (changeType == "critical") {
			const oldStyle = document.getElementById("__critical-css");
			const newStyle = document.createElement("style");
			newStyle.id = "__critical-css";
			newStyle.innerHTML = base64ToUTF8(criticalCSS);
			document.head.replaceChild(newStyle, oldStyle);
		}
		if (changeType == "revalidate") {
			console.log("KIRUNA DEV: Revalidating...");
			const el = document.getElementById("__refreshscript-rebuilding");
			if ("__kirunaRevalidate" in window) {
				__kirunaRevalidate().then(() => {
					console.log("KIRUNA DEV: Revalidated");
					if (el) el.remove();
				});
			} else {
				console.error("No __kirunaRevalidate() found");
				if (el) el.remove();
			}
		}
	};

	es.addEventListener("error", (e) => {
		console.log("KIRUNA DEV: SSE error", e);
		es.close();
		window.location.reload();
	});

	window.addEventListener("beforeunload", () => {
		es.close();
	});
`

func sseHandler(manager *clientManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		msg := make(chan refreshFilePayload, 1)
		client := &client{id: r.RemoteAddr, notify: msg}
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
					`{"changeType": "%s", "criticalCSS": "%s", "normalCSSURL": "%s", "at": "%s"}`,
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
