# 🏔️ Kiruna

### Kiruna is a lightweight dev server, CSS bundler, and static asset build tool for full-stack Go applications.

## Features

- **🚀 Automatic, optimized server rebuilds and browser refreshes during development**
- **🎨 Instant, Vite-style CSS hot reloading**
- **📦 Automatic CSS bunding and static asset hashing / embedding**
- **📝 Compatible with any Go templating engine**

## Quick note on project status and documentation quality

This project is in alpha stage. Some of the documentation is in comments in the code rather than below. You may need to piece some things together until I get the API finalized and the documentation cleaned up. Definitely feel free to reach out if you have any questions.

## Dev-time browser refreshes and Go application rebuilds

During development, Kiruna will update your browser in the fastest way possible, whether that's by (from fastest to slowest):

1. Hot reloading your CSS with no browser refresh at all;
2. Hard reloading the browser without rebuilding your server (for example, when you edit a template file); or
3. Fully rebuilding your Go server and hard reloading the browser.

This is configurable by telling Kiruna which file extensions to watch for changes, and how to handle those changes.

## Template strategy

By default, to get a tight feedback loop while building out your templates, changing a template file will only refresh your browser, and your Go application will not even be restarted. This works great if you parse your templates lazily (e.g., when a request comes in), but not as well if you parse your templates eagerly (e.g., on app startup). If you parse your templates eagerly, set `RestartApp` to `true` in your Kiruna config:

```go
// This is just a partial example specific to this topic. There's more to configure besides this.
&kiruna.Config{
  DevConfig: &kiruna.DevConfig{
    WatchedFiles: kiruna.WatchedFiles{
      ".go.html": {
        RestartApp: true,
      },
    },
  },
}
```

Kiruna is built to play nicely with Go's standard `html/template` package, but it should be flexible and unopinionated enough to work with any templating solution you may prefer.

## CSS hot reloading and critical CSS inlining

Assuming you follow a few simple conventions expected by the tool, Kiruna provides instant, Vite-style hot reloading for your bundled global CSS and inlined critical CSS. This provides a really great developer experience when iterating on styles.

Kiruna also provides a way to mark certain stylesheets as critical, so they can be automatically inlined into your document head.

Here are the conventions. Put a directory called `styles` in your Kiruna `RootDir` (this is probably also your project's root, but it doesn't necessarily have to be). Inside that directory, put two child directories: `critical` and `normal`. Inside of each of those files, you can write an unlimited number of `.css` files, and they will be bundled together (simply concatenated by alphabetical filename order). The bundled critical CSS can be inlined into your document head, and the bundled normal CSS can be served to the client via a standard link tag.

Note that hot reloading (with no browser refresh at all) only works if you write (or otherwise generate/output) your CSS in classic stylesheets, not inside of your markup. If, however, you write CSS in your markup (e.g., Tailwind), you can add in the Tailwind build step as a hook to your build process, and Kiruna will still provide instant browser refreshes when you edit your template file. To have Kiruna wait until Tailwind has finished building before refreshing your browser, you can hook into the build process by passing a custom `OnChange` function to your `WatchedFiles` map in the config.

Note that if you reference a local file in your CSS (e.g., `background-image: url(./my-image.png)`), it will automatically be converted to the hashed version served from your public folder.

## Static asset hashing and embedding

By default, Kiruna copies and hashes all of your static public assets (in `./static/public`), which lets you safely serve them with immutable cache headers, without worrying about asset mismatches or having to do full cache purges on deploy. If you have static assets that you don't want to have hashed, that is supported too; just put them in `./static/public/__nohash` instead of directly in `./static/public`.

Kiruna also handles your static private assets (such as template files or JSON data) in a similar way, but it does not hash them, since they are not served to the client. To have these included in your build, put them in `./static/private`. For example, your index template file might be at `./static/private/templates/index.tmpl`.

For production builds, static assets are embedded into your Go binary, which optimizes runtime performance and simplifies deployment. This will also work for any type of static asset you may want to serve, such as self-vendored JavaScript libraries or fonts. If you prefer to serve your assets from disk, you can do that too.

## Where exactly does Kiruna fit into the application lifecycle?

Kiruna's behavior can be broken down into four categories, with some overlap between them:

1. **Dev buildtime**
2. **Dev runtime**
3. **Prod buildtime**
4. **Prod runtime**

Visualized, you can think of it like this:

| Category        | **_Development_** | **_Production_** |
| --------------- | ----------------- | ---------------- |
| **_Buildtime_** | Dev buildtime     | Prod buildtime   |
| **_Runtime_**   | Dev runtime       | Prod runtime     |

Most behaviors are going to be identical between development and production, but there are some differences, such as the way that static assets are served.

Kiruna knows whether it is in development or production simply by whether it is called via `Kiruna.Dev()` or `Kiruna.Build()`.

Here's a rough overview of what is "special" about each category:

### Dev buildtime

Your development builds are initiated by your dev server, and they get outputted into your `dist` directory (which may be in your project root, or it may be in a sub-directory, depending on how you configure Kiruna). All of your static assets are hashed and copied into this directory, and your CSS is bundled and outputted into this directory as well. Your Go application is also built and outputted into this directory. All of this is identical between development and production, except that in dev the process is initiated by `Kiruna.Dev()` (rather than `Kiruna.Build()`).

Other than those superficial differences, only real difference between dev and prod buildtime is that the Kiruna dev server (which lives in its own process outside of your app and and is responsible for running development builds) will sometimes do only partial rebuilds in order to speed up the development feedback loop. It only does this when it's safe to do so based on how you configure Kiruna. The ability to do only partial rebuilds (for example, very quickly rebuilding your static assets without actually re-compiling your Go app) is possible because in your development _runtime_, your app reads templates and CSS files from disk, rather than from the embedded filesystem. In production, unless you configure it differently, your app reads templates and CSS files from the embedded filesystem, which means the Go app itself would need to be recompiled when you change static assets. The Go compiler is fast, but for certain dev-time changes, we can do better, and that's what the Kiruna dev server orchestrates and handles.

### Dev runtime

There are two "special" things about your development runtime:

1. As mentioned above, your app reads templates and CSS files from disk, so you can change them without rebuilding your app. This is the default behavior, but you can configure it differently if you want.

2. Your root HTML template will include a script tag that connects to the Kiruna dev server, which will allow it to receive messages from the dev server and refresh the browser when necessary, or just hot update the CSS when possible.

### Prod buildtime

Your production builds are identical to your development builds, except that they are manually triggered and have no connection to the Kiruna dev server.

### Prod runtime

Your production runtime is identical to your development runtime, except that your app reads templates and CSS files from the embedded filesystem, so you can't change them without rebuilding your app (this is default, but optional; if you want you can still use the os filesystem in production if you want). Additionally, you will not have the Kiruna dev server script tag in your root HTML template.

## Installation

```bash
go get -u github.com/sjc5/kiruna
```

## Dependencies

Kiruna has just a single dependency (`fsnotify`), which is used only at dev-time and will not be included in your final binary.

## Open source / closed contribution

For simplicity and reduced support load, Kiruna is currently open source / closed contribution. This may change in the future, but no promises.

## Copyright and License

Copyright 2024 Samuel J. Cook. Licensed under the BSD 3-Clause License.

## Alternatives

If you're just looking for automatic Go application rebuilds only, without automatic browser refreshes or static asset build tooling, then Kiruna may be overkill for you, and you could just use [Air](https://github.com/cosmtrek/air) instead.

That said, you can put Kiruna into a simpler `ServerOnly` mode if you want. This will disable all of the CSS and static asset build tooling, and it will only do automatic Go application rebuilds _a la_ Air.

One benefit (or downside, depending on your perspective) of Kiruna is that it doesn't require you to install any tooling on your machine. It just is orchestrated solely from inside your repo and its dependencies. So when a new developer joins your team, they can just clone your repo and be ready to rock as soon as they run `go mod tidy`, instead of needing to go install and configure Air.
