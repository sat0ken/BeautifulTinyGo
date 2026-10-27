//go:build tinygo && wasm

package main

import "wasmpreview/preview"

// setup for the browser preview. Use the size of the real panel so layout
// bugs show up here, not on the board.
func setup() (Screen, [reels]func() bool) {
	d := preview.NewDisplay(320, 240)
	return d, [reels]func() bool{
		preview.Button("ArrowLeft", "z", "a"),
		preview.Button("ArrowDown", "x", "b"),
		preview.Button("ArrowRight", "c"),
	}
}
