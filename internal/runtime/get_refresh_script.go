package runtime

import (
	"fmt"

	"github.com/sjc5/kiruna/internal/common"
)

func GetRefreshScript(config *common.Config) string {
	if !common.GetIsKirunaEnvDev() {
		return ""
	}

	return fmt.Sprintf(
		// changeTypes: "rebuilding", "other", "normal", "critical"
		// Element IDs: "__refreshscript-rebuilding", "__normal-css", "__critical-css"
		`<div id="__refreshscript-rebuilding" style="display: none;">Rebuilding...</div>
<script>
	const es = new EventSource("http://localhost:%d/events");
	es.onmessage = (e) => {
		const { changeType, criticalCss, normalCssUrl, at } = JSON.parse(e.data);
		if (changeType == "rebuilding") {
			console.log("Rebuilding server...");
			const el = document.getElementById("__refreshscript-rebuilding");
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
		}
		if (changeType == "other") {
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
			newStyle.innerHTML = criticalCss;
			document.head.replaceChild(newStyle, oldStyle);
		}
	};
</script>`,
		config.DevConfig.RefreshServerPort,
	)
}
