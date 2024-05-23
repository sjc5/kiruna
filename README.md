# 🏔️ Kiruna

## What is Kiruna?

Kiruna is an optimizing build tool and live refresh dev server for building full-stack Go applications. You can think of it as being a lot like Vite, but for Go.

### Dev server features

- Automatic rebuilds and browser refreshes
- Instant hot reloading for CSS files (without a full page refresh)

### Production optimizations

- Static asset hashing and embedding
- CSS bundling and minification
- Critical CSS inlining

Kiruna's static asset hashing strategy allows you to serve public static assets with immutable caching headers.

Dev-time reloads are smart and fast. Based on the type of file you edit and your associated configuration options, Kiruna will do the minimum amount of work necessary to get your changes to your browser as quickly as possible.

Kiruna has a few lightweight runtime helpers for referencing hashed static assets from Go code and templates (e.g., `Kiruna.GetPublicURL("favicon.ico")`) and for including your CSS in your HTML templates (e.g., `Kiruna.GetCriticalCSSStyleElement()`, `Kiruna.GetStyleSheetLinkElement()`). They have zero third-party dependencies are aggressively cached whenever possible, so you can feel free to call them even in the hot path of your application without much worry.

Kiruna is completely decoupled from any specific frameworks or libraries, so you can use it with any Go server framework or router you choose, or just use the standard library.

## Starter Tutorial From Scratch (~5 minutes)

Let's get a Kiruna project set up from scratch. This should only take a few minutes to complete. The only prerequisite is that you have Go installed on your machine.

### Scaffolding

Start by initializing a new Go module in an empty directory, replacing `your-module-name` with your own module name:

```sh
go mod init your-module-name
```

Then run the following commands to create the necessary directories and files for your project:

```sh
mkdir -p cmd/app && touch cmd/app/main.go && echo 'package main' > cmd/app/main.go
mkdir -p cmd/build && touch cmd/build/main.go && echo 'package main' > cmd/build/main.go
mkdir -p cmd/dev && touch cmd/dev/main.go && echo 'package main' > cmd/dev/main.go
mkdir -p styles/critical && touch styles/critical/main.css
mkdir -p styles/normal && touch styles/normal/main.css
mkdir -p dist/kiruna && touch dist/kiruna/x && touch dist/dist.go && echo "package dist" > dist/dist.go
mkdir -p static/private && touch static/private/index.go.html
mkdir -p static/public/__nohash
mkdir -p internal/platform && touch internal/platform/kiruna.go && echo "package platform" > internal/platform/kiruna.go
```

---

### Setup `dist/dist.go`

Now copy this into your `dist/dist.go` file, under the package declaration:

```go
import (
	"embed"
)

//go:embed kiruna
var FS embed.FS
```

---

### Setup `internal/platform/kiruna.go`

Now copy this into your `internal/platform/kiruna.go` file, under the package declaration, replacing `your-module-name` with your own module name:

```go
import (
	"your-module-name/dist"

	"github.com/sjc5/kiruna"
)

var Kiruna = kiruna.New(&kiruna.Config{
	DistFS:     dist.FS,
	EntryPoint: "cmd/app/main.go",
})
```

---

### go get Kiruna

Now go get Kiruna and tidy up:

```sh
go get github.com/sjc5/kiruna
go mod tidy
```

---

### Setup `static/private/index.go.html`

Now copy this into your `static/private/index.go.html` file:

```html
<!DOCTYPE html>
<html lang="en">
	<head>
		<meta charset="utf-8" />
		<meta name="viewport" content="width=device-width, initial-scale=1" />
		{{.Kiruna.GetCriticalCSSStyleElement}} {{.Kiruna.GetStyleSheetLinkElement}}
	</head>
	<body>
		<div id="root">
			<h1>Hello, world!</h1>
			<p>Hello from "static/private/index.go.html"</p>
		</div>
		{{.Kiruna.GetRefreshScript}}
	</body>
</html>
```

---

### Setup `cmd/app/main.go`

And now copy this into your `cmd/app/main.go` file, under the package declaration, replacing `your-module-name` with your own module name:

```go
import (
	"fmt"
	"html/template"
	"your-module-name/internal/platform"
	"net/http"

	"github.com/sjc5/kiruna"
)

func main() {
	// Health check endpoint
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Serve static files from "dist/kiruna/static/public" directory, accessible at "/public/"
	http.Handle("/public/", platform.Kiruna.GetServeStaticHandler("/public/", true))

	// Serve an HTML file using html/template
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		FS, err := platform.Kiruna.GetPrivateFS()
		if err != nil {
			fmt.Println(err)
			http.Error(w, "Error loading template", http.StatusInternalServerError)
			return
		}

		tmpl, err := template.ParseFS(FS, "index.go.html")
		if err != nil {
			fmt.Println(err)
			http.Error(w, "Error loading template", http.StatusInternalServerError)
			return
		}

		err = tmpl.Execute(w, struct {
			Kiruna     *kiruna.Kiruna
			FaviconURL string
		}{
			Kiruna:     platform.Kiruna,
			FaviconURL: platform.Kiruna.GetPublicURL("favicon.ico"),
		})
		if err != nil {
			http.Error(w, "Error executing template", http.StatusInternalServerError)
		}
	})

	port := kiruna.MustGetPort()

	fmt.Printf("Starting server on: http://localhost:%d\n", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
```

---

### Setup `cmd/build/main.go`

And copy this into your `cmd/build/main.go` file, under the package declaration, replacing `your-module-name` with your own module name:

```go
import (
	"your-module-name/internal/platform"
)

func main() {
	err := platform.Kiruna.Build()
	if err != nil {
		panic(err)
	}
}
```

This file is what you'll want to run when you're ready to build for production. Running `go run ./cmd/build` will build your project and save your binary to `dist/bin/main`. Assuming you used `DistFS` to embed your static assets, you can now run your binary from anywhere on the build machine, and it will serve your static assets from the embedded filesystem. If you chose not to embed your static assets, you'll just need to make sure that the binary is a sibling of the `dist/kiruna` directory in order to serve your static assets from disk.

**NOTE:** Oftentimes you'll want to handle compilation of your Go binary yourself. In such cases, you can use `platform.Kiruna.BuildWithoutCompilingGo()` instead of `platform.Kiruna.Build()`. This will run all the same Kiruna-specific processing (static asset hashing, etc.) but will stop short of producing an executable.

---

### Setup `cmd/dev/main.go`

Now copy this into your `cmd/dev/main.go` file, under the package declaration, replacing `your-module-name` with your own module name:

```go
import (
	"your-module-name/internal/platform"

	"github.com/sjc5/kiruna"
)

func main() {
	platform.Kiruna.MustStartDev(&kiruna.DevConfig{
		HealthcheckEndpoint: "/healthz",
		WatchedFiles:        kiruna.WatchedFiles{{Pattern: "**/*.go.html"}},
	})
}
```

---

### Run the dev server

Now try running the dev server:

```sh
go run ./cmd/dev
```

If you copied everything correctly, you should see some logging, with a link to your site on localhost, either at port `8080` or some fallback port. If you see an error, double check that you copied everything correctly.

---

### Edit critical CSS

Now paste the following into your `styles/critical/main.css` file, and hit save:

```css
body {
	background-color: darkblue;
	color: white;
}
```

If you leave your browser open and your dev server running, you should see the changes reflected in your browser nearly instantly via hot CSS reloading. Notice that the CSS above is being inlined into your document head. This is because it is in the `styles/critical` directory.

---

### Edit normal CSS

Now let's make sure your normal stylesheet is also working. Copy this into your `styles/normal/main.css` file:

```css
h1 {
	color: red;
}
```

When you hit save, this should also hot reload. Note that you can put multiple css stylesheets into both the `styles/critical` and `styles/normal` directories. In each case, the CSS will be minified and concatenated in alphabetical order by filename.

---

### Edit your html template

Now let's try editing your html template at `static/private/index.go.html`.

Find the line that says `<h1>Hello, world!</h1>` (line 10) and change it to: `<h1 style="color: green;">Hello, world!</h1>`.

When you hit save, your browser page should automatically refresh itself. This happens because of the `{Pattern: "**/*.go.html"}` item in the `kiruna.WatchedFiles` slice in `cmd/dev/main.go`. If you were to remove that item, the page would not reload when you save your html file (if you don't believe me, go give it a try).

When you want to watch different file types, you can add them to the `kiruna.WatchedFiles` slice using glob patterns, and there are a whole bunch of ways to tweak this to get your desired reload behavior and sequencing, including callbacks and more. Feel free to explore your auto-complete options here or dive into the Kiruna source code to learn more.

## Open source / closed contribution

For simplicity and reduced support load, especially while Kiruna is in active early development, Kiruna is currently open source / closed contribution. This may change in the future, but no promises.

## Copyright and License

Copyright 2024 Samuel J. Cook. Licensed under the BSD 3-Clause License.

## Alternatives

If you're just looking for automatic Go application rebuilds only, without automatic browser refreshes or static asset build tooling, then Kiruna may be overkill for you, and you could just use <a href="https://github.com/cosmtrek/air" target="_blank">Air</a> instead.

That said, you can put Kiruna into a simpler `ServerOnly` mode if you want. This will disable all of the CSS and static asset build tooling, and it will only do automatic Go application rebuilds _a la_ Air.

One benefit of Kiruna over Air is that it doesn't require you to install any tooling on your machine. It just is orchestrated solely from inside your repo and its dependencies. So when a new developer joins your team, they can just clone your repo and be ready to rock as soon as they run `go mod tidy`, instead of needing to install and configure Air first.
