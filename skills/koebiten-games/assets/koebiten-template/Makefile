# koebiten game helpers.
#
#   make wasm       build static/main.wasm and copy koebiten's page + wasm_exec.js
#   make capture    run it headless and write PNG snapshots (needs capture.cjs)
#   make serve      http://localhost:8000 for a human to play
#   make zero-kb02 / gopher-board / gopher-badge   build firmware and print the size
#   make flash TARGET=...                          flash one of them

.DEFAULT_GOAL := capture

# Evaluated when a recipe runs, after go.sum exists and the module is downloaded.
KOEBITEN = $(shell go list -m -f '{{.Dir}}' github.com/sago35/koebiten)
CAPTURE  ?= tools/capture.cjs
AT       ?= 500,2500
KEYS     ?= 300:z:80,1200:ArrowLeft:400,1900:ArrowRight:400
PORT     ?= 8000

# go.sum is not shipped with the template; create it on first use.
go.sum: go.mod
	go mod tidy

wasm: go.sum
	mkdir -p static
	tinygo build -o static/main.wasm -target=wasm -no-debug -panic=trap .
	cp "$$(tinygo env TINYGOROOT)/targets/wasm_exec.js" static/
	cp $(KOEBITEN)/static/index.html $(KOEBITEN)/static/main.js $(KOEBITEN)/static/screen-emulator.js static/
	chmod u+w static/*

capture: wasm
	node $(CAPTURE) static/main.wasm --duration 3000 --at $(AT) --keys "$(KEYS)"

serve: wasm
	python3 -m http.server $(PORT) --directory static

zero-kb02: go.sum
	tinygo build -o out/zero-kb02.uf2 -size short -target $(KOEBITEN)/targets/zero-kb02.json .

gopher-board: go.sum
	tinygo build -o out/gopher-board-spi.uf2 -size short -target $(KOEBITEN)/targets/gopher-board-spi.json .

gopher-badge: go.sum
	tinygo build -o out/gopher-badge.uf2 -size short -target gopher-badge .

TARGET ?= $(KOEBITEN)/targets/zero-kb02.json
flash: go.sum
	tinygo flash -size short -target $(TARGET) .

.PHONY: wasm capture serve zero-kb02 gopher-board gopher-badge flash
