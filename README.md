# [astrophena.name](https://astrophena.name)

This is my personal site and a [Go] package that generates it.

## Serving locally

You need the latest [Go] installed.

```sh
$ git clone https://github.com/astrophena/site
$ cd site
$ ./serve.go
```

Open http://localhost:3000 in your browser.

## Deploying

[GitHub Actions](https://github.com/actions) automatically deploys each commit
in master branch. To deploy [go.astrophena.name](https://go.astrophena.name),
run:

```sh
$ gh workflow run deploy.yml -R astrophena/vanity
```

## License

The content for this website is
[CC-BY](https://creativecommons.org/licenses/by/4.0/), the code is
[ISC](https://opensource.org/licenses/ISC).

[go]: https://go.dev
