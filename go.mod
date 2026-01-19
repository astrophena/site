module go.astrophena.name/site

go 1.25.5

require (
	github.com/PuerkitoBio/goquery v1.11.0
	github.com/fsnotify/fsnotify v1.9.0
	github.com/gorilla/feeds v1.2.0
	github.com/tdewolff/minify/v2 v2.24.8
	go.astrophena.name/base v0.16.1
	go.starlark.net v0.0.0-20250906160240-bf296ed553ea
	rsc.io/markdown v0.0.0-20241212154241-6bf72452917f
)

require (
	braces.dev/errtrace v0.3.0 // indirect
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/alecthomas/chroma/v2 v2.14.0 // indirect
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/dlclark/regexp2 v1.11.4 // indirect
	github.com/fluhus/godoc-tricks v1.5.0 // indirect
	github.com/go4org/hashtriemap v0.0.0-20251130024219-545ba229f689 // indirect
	github.com/lmittmann/tint v1.1.2 // indirect
	github.com/peterbourgon/ff/v3 v3.4.0 // indirect
	github.com/tdewolff/parse/v2 v2.8.5 // indirect
	go.abhg.dev/doc2go v0.8.2-0.20240626042920-4345d7c36b95 // indirect
	go.astrophena.name/tools v1.2.1-0.20250925115459-1c70e3e58329 // indirect
	golang.org/x/exp/typeparams v0.0.0-20251219203646-944ab1f22d93 // indirect
	golang.org/x/mod v0.31.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/term v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	golang.org/x/tools v0.40.0 // indirect
	golang.org/x/tools/go/packages/packagestest v0.1.1-deprecated // indirect
	honnef.co/go/tools v0.6.1 // indirect
)

tool (
	go.astrophena.name/base/devtools/addcopyright
	go.astrophena.name/base/devtools/pre-commit
	go.astrophena.name/site/internal/devtools/build
	go.astrophena.name/site/internal/devtools/resize-icons
	go.astrophena.name/site/internal/devtools/serve
	go.astrophena.name/tools/cmd/deploy
)

tool (
	go.abhg.dev/doc2go
	honnef.co/go/tools/cmd/staticcheck
)
