//go:build tinygo

// Command example shows how a ported Arduboy game sits on koebiten:
// the game draws on an arduboy.Canvas with Arduboy2 semantics and flushes it
// to koebiten's display once per frame; Arduboy buttons map to koebiten keys.
//
// The "game" is a ball sprite (Sprites plus-mask format, copied as bytes the
// way a PROGMEM array would be) bouncing around a frame. A inverts the
// screen, the d-pad pushes the ball.
package main

import (
	"image/color"

	"arduboycompat/arduboy"

	"github.com/sago35/koebiten"
	"github.com/sago35/koebiten/hardware"
	"tinygo.org/x/drivers/pixel"
	"tinygo.org/x/tinyfont"
)

// ball is 8x8 in Sprites::drawPlusMask format: width, height, then
// (image, mask) byte pairs, one vertical byte per column.
var ball = []byte{8, 8,
	0x00, 0x3c, 0x3c, 0x7e, 0x7e, 0xff, 0x7e, 0xff,
	0x7e, 0xff, 0x7a, 0xff, 0x3c, 0x7e, 0x00, 0x3c,
}

// Arduboy button bits (Arduboy2Core.h) mapped to koebiten keys.
const (
	upButton    = 1 << 7
	downButton  = 1 << 4
	leftButton  = 1 << 5
	rightButton = 1 << 6
	aButton     = 1 << 3
	bButton     = 1 << 2
)

var keyFor = [...]struct {
	bit uint8
	key koebiten.Key
}{
	{upButton, koebiten.KeyUp}, {downButton, koebiten.KeyDown},
	{leftButton, koebiten.KeyLeft}, {rightButton, koebiten.KeyRight},
	{aButton, koebiten.Key0}, {bButton, koebiten.Key1},
}

// pressed is Arduboy2 pressed(buttons): true when all given buttons are held.
func pressed(buttons uint8) bool {
	for _, k := range keyFor {
		if buttons&k.bit != 0 && !koebiten.IsKeyPressed(k.key) {
			return false
		}
	}
	return true
}

// justPressed is Arduboy2 justPressed(button) for a single button.
func justPressed(button uint8) bool {
	for _, k := range keyFor {
		if button == k.bit {
			return koebiten.IsKeyJustPressed(k.key)
		}
	}
	return false
}

type game struct {
	canvas arduboy.Canvas
	set    func(x, y int16, c color.RGBA) // the koebiten display's SetPixel
	x, y   int                            // in 1/16 pixels
	dx, dy int
	invert bool
	frames uint32
}

var (
	ink   = pixel.NewMonochrome(0x00, 0x00, 0x00)
	paper = pixel.NewMonochrome(0xff, 0xff, 0xff)
)

// The Arduboy original ran at 60 FPS; koebiten ticks at about 31. Speeds per
// frame are doubled so the ball crosses the screen in the same time.
const speedScale = 2

func (g *game) Update() error {
	g.frames++
	if justPressed(aButton) {
		g.invert = !g.invert
	}
	if pressed(leftButton) {
		g.dx -= 2
	}
	if pressed(rightButton) {
		g.dx += 2
	}
	if pressed(upButton) {
		g.dy -= 2
	}
	if pressed(downButton) {
		g.dy += 2
	}
	g.x += g.dx * speedScale
	g.y += g.dy * speedScale
	if g.x < 16 || g.x > (arduboy.Width-9)*16 {
		g.dx = -g.dx
		g.x += g.dx * speedScale
	}
	if g.y < 16 || g.y > (arduboy.Height-9)*16 {
		g.dy = -g.dy
		g.y += g.dy * speedScale
	}
	return nil
}

func (g *game) Draw(screen *koebiten.Image) {
	c := &g.canvas
	c.Clear()
	c.DrawRect(0, 0, arduboy.Width, arduboy.Height, arduboy.White)
	c.DrawPlusMask(g.x/16, g.y/16, ball, 0)
	if g.invert {
		c.FillRect(1, 1, arduboy.Width-2, arduboy.Height-2, arduboy.Invert)
	}
	// Arduboy White is a lit pixel; on koebiten that is ink on the white page
	// koebiten already cleared, so only lit pixels are sent (unlit.A == 0).
	c.Flush(g.set, ink.RGBA(), color.RGBA{})
	// Text after the flush, with tinyfont instead of Arduboy's 5x7 font.
	text := ink
	if g.invert {
		text = paper
	}
	koebiten.DrawText(screen, "A: INVERT", &tinyfont.Org01, 3, 61, text)
}

func (g *game) Layout(int, int) (int, int) { return arduboy.Width, arduboy.Height }

func main() {
	koebiten.SetHardware(hardware.Device)
	g := &game{x: 40 * 16, y: 20 * 16, dx: 11, dy: 7}
	// Draw straight to the display: koebiten's Draw* helpers check for a nil
	// screen with reflection on every call, too slow for per-pixel use.
	g.set = hardware.Device.GetDisplay().SetPixel
	koebiten.RunGame(g)
}
