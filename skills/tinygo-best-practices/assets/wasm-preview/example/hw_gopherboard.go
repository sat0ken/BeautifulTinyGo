//go:build waveshare_rp2040_zero

package main

import (
	"machine"

	"tinygo.org/x/drivers/st7789"
)

// setup for gopher-board v2 (Waveshare RP2040-Zero + ST7789 240x240), using
// the pins from koebiten's hardware/gopher-board-spi.go. Change the pins for
// another board and keep the same function signature.
func setup() (Screen, [reels]func() bool) {
	machine.SPI1.Configure(machine.SPIConfig{Frequency: 16_000_000, Mode: 0})
	d := st7789.New(machine.SPI1,
		machine.GPIO9,  // reset
		machine.GPIO12, // dc
		machine.GPIO13, // cs
		machine.GPIO14) // backlight
	d.Configure(st7789.Config{Width: 240, Height: 240, Rotation: st7789.ROTATION_90})

	var buttons [reels]func() bool
	for i, p := range [reels]machine.Pin{machine.GPIO4, machine.GPIO6, machine.GPIO5} { // left, down, right
		p.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
		buttons[i] = func() bool { return !p.Get() }
	}
	return &d, buttons
}
