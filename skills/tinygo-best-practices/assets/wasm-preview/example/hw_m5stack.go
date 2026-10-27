//go:build m5stack

package main

import (
	"machine"

	"tinygo.org/x/drivers/ili9341"
)

// setup for M5Stack Basic/Gray (tinygo -target=m5stack): ILI9341 320x240 and
// the three front buttons A/B/C, which read low while pressed.
func setup() (Screen, [reels]func() bool) {
	machine.SPI2.Configure(machine.SPIConfig{
		SCK:       machine.SPI0_SCK_PIN,
		SDO:       machine.SPI0_SDO_PIN,
		SDI:       machine.SPI0_SDI_PIN,
		Frequency: 40e6,
	})
	bl := machine.LCD_BL_PIN
	bl.Configure(machine.PinConfig{Mode: machine.PinOutput})
	d := ili9341.NewSPI(machine.SPI2, machine.LCD_DC_PIN, machine.LCD_SS_PIN, machine.LCD_RST_PIN)
	d.Configure(ili9341.Config{Width: 320, Height: 240, DisplayInversion: true})
	d.SetRotation(ili9341.Rotation0Mirror)
	bl.High()

	var buttons [reels]func() bool
	for i, p := range [reels]machine.Pin{machine.BUTTON_A, machine.BUTTON_B, machine.BUTTON_C} {
		p.Configure(machine.PinConfig{Mode: machine.PinInput})
		buttons[i] = func() bool { return !p.Get() }
	}
	return d, buttons
}
