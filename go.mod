module go.astrophena.name/site

go 1.24

require (
	github.com/PuerkitoBio/goquery v1.10.3
	github.com/fsnotify/fsnotify v1.9.0
	github.com/gorilla/feeds v1.2.0
	go.astrophena.name/base v0.6.0
	go.starlark.net v0.0.0-20240925182052-1207426daebd
	rsc.io/markdown v0.0.0-20241212154241-6bf72452917f
)

require (
	braces.dev/errtrace v0.3.0 // indirect
	github.com/BurntSushi/toml v1.4.1-0.20240526193622-a339e1f7089c // indirect
	github.com/alecthomas/chroma/v2 v2.14.0 // indirect
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/dlclark/regexp2 v1.11.4 // indirect
	github.com/fluhus/godoc-tricks v1.5.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/peterbourgon/ff/v3 v3.4.0 // indirect
	go.abhg.dev/doc2go v0.8.2-0.20240626042920-4345d7c36b95 // indirect
	golang.org/x/exp/typeparams v0.0.0-20231108232855-2478ac86f678 // indirect
	golang.org/x/mod v0.24.0 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	golang.org/x/tools v0.31.0 // indirect
	honnef.co/go/tools v0.6.1 // indirect
)

tool (
	go.astrophena.name/site/internal/devtools/addcopyright
	go.astrophena.name/site/internal/devtools/build
	go.astrophena.name/site/internal/devtools/pre-commit
	go.astrophena.name/site/internal/devtools/serve
)

tool (
	go.abhg.dev/doc2go
	honnef.co/go/tools/cmd/staticcheck
)
