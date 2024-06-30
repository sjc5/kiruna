package ik

import (
	"fmt"
	"net/http"
	"time"
)

func GetRefreshScript(config *Config) string {
	if !KirunaEnv.GetIsDev() {
		return ""
	}
	inner := GetRefreshScriptInner(KirunaEnv.getRefreshServerPort())
	return "\n<script>\n" + inner + "\n</script>"
}

func GetRefreshScriptInner(port int) string {
	return fmt.Sprintf(refreshScriptFmt, port)
}

// changeTypes: "rebuilding", "other", "normal", "critical"
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
	const { changeType, criticalCss, normalCssUrl, at } = JSON.parse(e.data);
	if (changeType == "rebuilding") {
		console.log("Rebuilding server...");
		const el = document.createElement("div");
		el.innerHTML = "Rebuilding...";
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
		newLink.href = normalCssUrl;
		newLink.onload = () => oldLink.remove();
		oldLink.parentNode.insertBefore(newLink, oldLink.nextSibling);
	}
	if (changeType == "critical") {
		const oldStyle = document.getElementById("__critical-css");
		const newStyle = document.createElement("style");
		newStyle.id = "__critical-css";
		newStyle.innerHTML = base64ToUTF8(criticalCss);
		document.head.replaceChild(newStyle, oldStyle);
	}
};

es.addEventListener("error", (e) => {
	console.log("SSE error", e);
	es.close();
	window.location.reload();
});

window.addEventListener("beforeunload", () => {
	es.close();
});
`

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
