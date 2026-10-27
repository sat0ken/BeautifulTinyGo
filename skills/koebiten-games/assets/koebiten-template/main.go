package main

import (
	"mygame/game"

	"github.com/sago35/koebiten"
	"github.com/sago35/koebiten/hardware"
)

func main() {
	// hardware.Device is chosen by build tags: zero_kb02, gopher_board_spi,
	// gopher-badge, pybadge, wioterminal, macropad-rp2040, ... or wasm.
	koebiten.SetHardware(hardware.Device)
	koebiten.SetWindowSize(game.W, game.H)
	koebiten.SetWindowTitle("Dodge")

	if err := koebiten.RunGame(game.New()); err != nil {
		println(err.Error())
	}
}
