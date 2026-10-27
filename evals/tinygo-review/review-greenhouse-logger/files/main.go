// Greenhouse logger for a Raspberry Pi Pico: shows temperature and humidity
// on a 240x240 ST7789, prints a log line over USB serial every second, and
// counts presses of the button on GP14.
package main

import (
	"fmt"
	"image/color"
	"machine"
	"strconv"
	"time"

	"example.com/greenhouse/si7021"
	"tinygo.org/x/drivers/st7789"
	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/freemono"
)

var (
	presses int
	pressed bool
	history = map[string]float64{}
	logbook []string
)

var (
	black = color.RGBA{0, 0, 0, 255}
	white = color.RGBA{255, 255, 255, 255}
	green = color.RGBA{0, 200, 80, 255}
)

func main() {
	machine.I2C0.Configure(machine.I2CConfig{SDA: machine.GP4, SCL: machine.GP5, Frequency: 400 * machine.KHz})
	sensor := si7021.New(machine.I2C0, machine.GP22, si7021.WithHeater(false))
	if err := sensor.Configure(); err != nil {
		println(err.Error())
	}

	machine.SPI1.Configure(machine.SPIConfig{Frequency: 62_500_000, SCK: machine.GP10, SDO: machine.GP11})
	display := st7789.New(machine.SPI1, machine.GP12, machine.GP8, machine.GP9, machine.GP13)
	display.Configure(st7789.Config{Width: 240, Height: 240, Rotation: st7789.ROTATION_90})

	button := machine.GP14
	button.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	button.SetInterrupt(machine.PinFalling, func(machine.Pin) {
		presses++
		pressed = true
		logbook = append(logbook, "press "+strconv.Itoa(presses))
		println("button pressed", presses)
	})

	machine.InitADC()
	battery := machine.ADC{Pin: machine.ADC2}
	battery.Configure(machine.ADCConfig{})

	go blink()

	for {
		rh, err := sensor.ReadHumidity()
		if err != nil {
			println(err.Error())
			continue
		}
		t := sensor.Temperature()
		volts := float64(battery.Get()) / 4095 * 3.3 * 2 // 1:2 divider on the battery

		line := fmt.Sprintf("T=%.1fC RH=%.0f%% bat=%.2fV presses=%d", t, rh, volts, presses)
		println(line)
		history[time.Now().String()] = t
		logbook = append(logbook, line)

		draw(&display, t, rh, pressed)
		pressed = false
		time.Sleep(time.Second)
	}
}

// blink toggles the on-board LED as a heartbeat.
func blink() {
	led := machine.LED
	led.Configure(machine.PinConfig{Mode: machine.PinOutput})
	for {
		led.High()
		for i := 0; i < 2_000_000; i++ {
		}
		led.Low()
		for i := 0; i < 2_000_000; i++ {
		}
	}
}

// draw shows the readings: the temperature, the humidity and a humidity bar.
func draw(d *st7789.Device, t, rh float64, pressed bool) {
	// Clear the whole screen, then draw the humidity bar pixel by pixel.
	for y := int16(0); y < 240; y++ {
		for x := int16(0); x < 240; x++ {
			d.SetPixel(x, y, black)
		}
	}
	w := int16(rh * 2.2)
	for y := int16(150); y < 180; y++ {
		for x := int16(10); x < 10+w; x++ {
			d.SetPixel(x, y, green)
		}
	}
	tinyfont.WriteLine(d, &freemono.Bold18pt7b, 10, 60, strconv.FormatFloat(t, 'f', 1, 64)+" C", white)
	tinyfont.WriteLine(d, &freemono.Bold18pt7b, 10, 120, strconv.Itoa(int(rh))+" %", white)
	if pressed {
		tinyfont.WriteLine(d, &freemono.Bold12pt7b, 10, 220, "BUTTON!", white)
	}
}
