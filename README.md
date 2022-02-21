# [astrophena.name](https://astrophena.name)

[![Go Documentation](https://godocs.io/go.astrophena.name/site?status.svg)](https://godocs.io/go.astrophena.name/site)

This is my personal website, hosted on [GitHub Pages](https://pages.github.com).

## Serving locally

You need the latest [Go](https://go.dev) and [Node.js](https://nodejs.org)
installed.

```sh
$ git clone https://github.com/astrophena/astrophena.github.io.git
$ cd astrophena.github.io
$ npm install
$ ./build.go -serve localhost:3000
```

Open http://localhost:3000 in your browser.

## Style

All code in this repository are formatted by:

- [gofmt](https://godocs.io/cmd/gofmt) (Go)
- [prettier](https://prettier.io) (Markdown, HTML and CSS)

Run `npm run fmt` to do this. `npm run check` verifies if the code is correctly
formatted.

## License

The content for this website is
[CC-BY](https://creativecommons.org/licenses/by/4.0/), the
[code](https://github.com/astrophena/astrophena.github.io) is
[MIT](https://opensource.org/licenses/MIT).
