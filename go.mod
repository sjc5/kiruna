module github.com/sjc5/kiruna

go 1.22.0

replace github.com/sjc5/kit => ../go-kit

require (
	github.com/bmatcuk/doublestar/v4 v4.6.1
	github.com/fsnotify/fsnotify v1.7.0
	github.com/sjc5/kit v0.0.15
)

require golang.org/x/sys v0.17.0 // indirect
