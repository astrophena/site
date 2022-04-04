# [astrophena.name](https://astrophena.name)

[![Go Documentation](https://godocs.io/go.astrophena.name/site?status.svg)](https://godocs.io/go.astrophena.name/site)

This is my personal website, hosted on [GitHub Pages](https://pages.github.com).

## Serving locally

You need the latest [Go] installed.

```sh
$ git clone https://github.com/astrophena/astrophena.github.io.git site
$ cd site
$ script/build
```

Open http://localhost:3000 in your browser.

## Style

All code in this repository are formatted by:

- [gofmt](https://godocs.io/cmd/gofmt) ([Go])
- [shfmt](https://godocs.io/mvdan.cc/sh/v3/cmd/shfmt) (shell scripts)
- [Prettier] (Markdown, HTML and CSS)

[Node.js](https://nodejs.org) is required to run [Prettier].

Run `script/test` to format everything. CI verifies if the code is correctly formatted.

## License

The content for this website is [CC-BY](https://creativecommons.org/licenses/by/4.0/), the
[code](https://github.com/astrophena/astrophena.github.io) is
[MIT](https://opensource.org/licenses/MIT).

[go]: https://go.dev
[prettier]: https://prettier.io
