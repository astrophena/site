module go.astrophena.name/site

go 1.24

require (
	github.com/PuerkitoBio/goquery v1.10.2
	github.com/fsnotify/fsnotify v1.8.0
	github.com/gorilla/feeds v1.2.0
	go.astrophena.name/base v0.4.0
	go.starlark.net v0.0.0-20240925182052-1207426daebd
	rsc.io/markdown v0.0.0-20241212154241-6bf72452917f
)

require (
	braces.dev/errtrace v0.3.0 // indirect
	github.com/alecthomas/chroma/v2 v2.14.0 // indirect
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/dlclark/regexp2 v1.11.4 // indirect
	github.com/fluhus/godoc-tricks v1.5.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/peterbourgon/ff/v3 v3.4.0 // indirect
	go.abhg.dev/doc2go v0.8.2-0.20240626042920-4345d7c36b95 // indirect
	golang.org/x/mod v0.24.0 // indirect
	golang.org/x/net v0.37.0 // indirect
	golang.org/x/sync v0.12.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	golang.org/x/tools v0.31.0 // indirect
)

tool (
	go.astrophena.name/site/internal/tools/build
	go.astrophena.name/site/internal/tools/serve
)

tool go.abhg.dev/doc2go
