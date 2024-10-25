build:
	@ GOOS=js GOARCH=wasm go generate
	@ GOOS=js GOARCH=wasm go build -o star.wasm

clean:
	@ rm -f star.wasm

.PHONY: build clean
