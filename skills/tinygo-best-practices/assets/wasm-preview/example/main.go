// Command example is a three-reel "stop the digits" toy that shows the
// layout BeautifulTinyGo recommends for display programs:
//
//   - main.go (this file) holds all logic and drawing. It never imports
//     "machine"; it only sees a Screen and buttons as func() bool.
//   - hw_*.go files build the real display and buttons for one board each.
//   - hw_wasm.go builds a canvas display and keyboard buttons for the
//     browser preview (tinygo build -target=wasm).
package main

import (
	"image/color"
	"time"

	"tinygo.org/x/drivers"
	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/freemono"
)

// Screen is what the game needs from a display. st7789, ili9341, ssd1306 and
// preview.Display all satisfy it.
type Screen interface {
	drivers.Displayer
	FillRectangle(x, y, width, height int16, c color.RGBA) error
}

const (
	reels     = 3
	frameTime = 33 * time.Millisecond
	digits    = "0123456789"
)

var (
	black = color.RGBA{0x00, 0x00, 0x00, 0xff}
	white = color.RGBA{0xff, 0xff, 0xff, 0xff}
	gold  = color.RGBA{0xff, 0xc8, 0x2e, 0xff}
	gray  = color.RGBA{0x40, 0x40, 0x48, 0xff}
)

type game struct {
	value    [reels]uint8
	stopped  [reels]bool
	prevBtn  [reels]bool
	tick     uint8
	dirty    [reels]bool // only redraw what changed: SPI displays are slow
	allDirty bool
}

func (g *game) reset() {
	for i := range g.stopped {
		g.stopped[i] = false
		g.dirty[i] = true
	}
	g.allDirty = true
}

func (g *game) done() bool {
	for _, s := range g.stopped {
		if !s {
			return false
		}
	}
	return true
}

func (g *game) win() bool {
	return g.value[0] == g.value[1] && g.value[1] == g.value[2]
}

// update advances one frame. buttons[i] stops reel i; any button restarts
// the round once every reel has stopped.
func (g *game) update(buttons [reels]func() bool) {
	var pressed [reels]bool
	for i, b := range buttons {
		now := b()
		pressed[i] = now && !g.prevBtn[i] // rising edge only
		g.prevBtn[i] = now
	}
	if g.done() {
		if pressed[0] || pressed[1] || pressed[2] {
			g.reset()
		}
		return
	}
	for i := range pressed {
		if pressed[i] && !g.stopped[i] {
			g.stopped[i] = true
			g.dirty[i] = true
			if g.done() {
				g.allDirty = true
			}
		}
	}
	g.tick++
	if g.tick%3 != 0 {
		return
	}
	for i := range g.value {
		if !g.stopped[i] {
			g.value[i] = (g.value[i] + uint8(i) + 1) % 10
			g.dirty[i] = true
		}
	}
}

func (g *game) draw(s Screen) {
	w, h := s.Size()
	reelW := w / 4
	gap := (w - reelW*reels) / (reels + 1)
	top := h/2 - reelW/2
	if g.allDirty {
		s.FillRectangle(0, 0, w, h, black)
		msg, c := "PRESS", white
		if g.done() && g.win() {
			msg, c = "WIN!", gold
		} else if g.done() {
			msg, c = "AGAIN", white
		}
		tinyfont.WriteLine(s, &freemono.Bold12pt7b, gap, top+reelW+30, msg, c)
		for i := range g.dirty {
			g.dirty[i] = true
		}
		g.allDirty = false
	}
	for i := range g.value {
		if !g.dirty[i] {
			continue
		}
		x := gap + int16(i)*(reelW+gap)
		bg := gray
		if g.stopped[i] {
			bg = gold
		}
		s.FillRectangle(x, top, reelW, reelW, bg)
		d := digits[g.value[i] : g.value[i]+1] // slicing a string does not allocate
		tinyfont.WriteLine(s, &freemono.Bold18pt7b, x+reelW/2-10, top+reelW/2+10, d, black)
		g.dirty[i] = false
	}
}

func main() {
	screen, buttons := setup() // provided by hw_*.go
	g := &game{}
	g.reset()
	for {
		g.update(buttons)
		g.draw(screen)
		screen.Display() // a no-op on st7789/ili9341, required for buffered displays and the preview
		time.Sleep(frameTime)
	}
}
