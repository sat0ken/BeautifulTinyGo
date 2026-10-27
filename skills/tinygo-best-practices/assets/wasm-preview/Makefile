# Display preview helpers. Copy into your project and change PKG / SIZE / KEYS.
#
#   make wasm      build web/main.wasm and copy TinyGo's wasm_exec.js next to it
#   make capture   run it headless in Node and write PNG snapshots to preview-out/
#   make serve     serve web/ on http://localhost:8080 for a human to play with
#   make hw        build for the real board (TARGET) and print the size

.DEFAULT_GOAL := capture

PKG     ?= ./example
TARGET  ?= waveshare-rp2040-zero
AT      ?= 300,1100,1900
KEYS    ?= 500:ArrowLeft:60,900:ArrowDown:60,1300:ArrowRight:60
PORT    ?= 8080

# go.sum is not shipped with the template; create it on first use.
go.sum: go.mod
	go mod tidy

wasm: go.sum
	tinygo build -o web/main.wasm -target=wasm -no-debug $(PKG)
	cp "$$(tinygo env TINYGOROOT)/targets/wasm_exec.js" web/

capture: wasm
	node tools/capture.cjs web/main.wasm --at $(AT) --keys "$(KEYS)"

serve: wasm
	python3 -m http.server $(PORT) --directory web

hw: go.sum
	tinygo build -o out.uf2 -target=$(TARGET) -size short $(PKG)

.PHONY: wasm capture serve hw
