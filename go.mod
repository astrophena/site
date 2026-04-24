module go.astrophena.name/site

go 1.26.2

require (
	github.com/PuerkitoBio/goquery v1.12.0
	github.com/fsnotify/fsnotify v1.9.0
	github.com/gorilla/feeds v1.2.0
	github.com/tdewolff/minify/v2 v2.24.12
	go.astrophena.name/base v0.19.0
	go.starlark.net v0.0.0-20260326113308-fadfc96def35
	golang.org/x/sync v0.20.0
	rsc.io/markdown v0.0.0-20241212154241-6bf72452917f
)

require (
	braces.dev/errtrace v0.4.0 // indirect
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/alecthomas/chroma/v2 v2.23.1 // indirect
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/dlclark/regexp2 v1.11.5 // indirect
	github.com/fluhus/godoc-tricks v1.6.0 // indirect
	github.com/go4org/hashtriemap v0.0.0-20251130024219-545ba229f689 // indirect
	github.com/lmittmann/tint v1.1.3 // indirect
	github.com/peterbourgon/ff/v3 v3.4.0 // indirect
	github.com/tdewolff/parse/v2 v2.8.11 // indirect
	go.abhg.dev/doc2go v0.12.1 // indirect
	golang.org/x/exp/typeparams v0.0.0-20260218203240-3dfff04db8fa // indirect
	golang.org/x/mod v0.35.0 // indirect
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/term v0.42.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	golang.org/x/tools v0.44.0 // indirect
	honnef.co/go/tools v0.7.0 // indirect
)

tool (
	go.astrophena.name/base/devtools/addcopyright
	go.astrophena.name/base/devtools/deploy
	go.astrophena.name/base/devtools/pre-commit
	go.astrophena.name/site/internal/devtools/build
	go.astrophena.name/site/internal/devtools/resize-icons
	go.astrophena.name/site/internal/devtools/serve
)

tool (
	go.abhg.dev/doc2go
	honnef.co/go/tools/cmd/staticcheck
)
