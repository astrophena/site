<!--
© 2025 Ilya Mateyko. All rights reserved.
Use of this source code is governed by the CC-BY
license that can be found in the LICENSE.md file.
-->

# [astrophena.name](https://astrophena.name)

This is my personal website.

## Development

You need the latest version of [Go] installed.

First, clone the repository:

```sh
$ git clone https://github.com/astrophena/site
$ cd site
```

To serve the site locally, run:

```sh
$ go tool serve
```

This command starts a development server at `http://localhost:3000` and
automatically rebuilds the site when files in the `pages`, `static`, or
`templates` directories are changed.

To generate a production-ready build, use:

```sh
$ go tool build -prod
```

The static files will be placed in the `build/` directory.

To set up the Git pre-commit hook for development:

```sh
$ go tool pre-commit
```

## License

The content for this website is
[CC-BY](https://creativecommons.org/licenses/by/4.0/), the code is
[ISC](https://opensource.org/licenses/ISC).

[go]: https://go.dev
