// Package arduboy reproduces the drawing model of the Arduboy2 library in Go
// so that Arduboy games can be ported to TinyGo (usually on top of koebiten)
// with their drawing code translated line by line.
//
// The Canvas keeps a 128x64 one-bit frame buffer with the same layout as
// Arduboy2's sBuffer: 8 pages of 128 bytes, each byte a vertical column of 8
// pixels with bit 0 at the top. Bitmaps and sprites copied from a game's
// PROGMEM arrays therefore draw exactly as they did on the Arduboy, including
// masks and INVERT. Call Flush once per frame to put the result on any
// display that has SetPixel (koebiten, drivers.Displayer, the WebAssembly
// preview).
//
// Colours follow Arduboy: White is a lit pixel, Black is background. Flush
// decides how lit pixels look on the target.
//
// The package has no dependencies beyond image/color, so it is tested with the
// normal `go test`. It allocates nothing after the Canvas is created.
package arduboy

import "image/color"

// Screen size of the Arduboy.
const (
	Width  = 128
	Height = 64
)

// Color is an Arduboy drawing colour.
type Color uint8

// Arduboy2 colours.
const (
	Black  Color = 0 // background
	White  Color = 1 // lit pixel
	Invert Color = 2 // flip the existing pixel
)

// Canvas is an Arduboy-compatible frame buffer.
type Canvas struct {
	buf [Width * Height / 8]byte
}

// Clear sets every pixel to Black (Arduboy2 clear()).
func (c *Canvas) Clear() {
	for i := range c.buf {
		c.buf[i] = 0
	}
}

// FillScreen sets every pixel to col (Arduboy2 fillScreen()). Invert is
// treated as White, as in Arduboy2.
func (c *Canvas) FillScreen(col Color) {
	v := byte(0)
	if col != Black {
		v = 0xff
	}
	for i := range c.buf {
		c.buf[i] = v
	}
}

// Buffer exposes the raw frame buffer (Arduboy2 getBuffer()).
func (c *Canvas) Buffer() *[Width * Height / 8]byte {
	return &c.buf
}

// DrawPixel sets one pixel. Pixels outside the screen are ignored.
func (c *Canvas) DrawPixel(x, y int, col Color) {
	if x < 0 || x >= Width || y < 0 || y >= Height {
		return
	}
	i := (y/8)*Width + x
	bit := byte(1) << uint(y%8)
	switch col {
	case White:
		c.buf[i] |= bit
	case Black:
		c.buf[i] &^= bit
	case Invert:
		c.buf[i] ^= bit
	}
}

// GetPixel returns White or Black (Arduboy2 getPixel()). Outside the screen it
// returns Black.
func (c *Canvas) GetPixel(x, y int) Color {
	if x < 0 || x >= Width || y < 0 || y >= Height {
		return Black
	}
	if c.buf[(y/8)*Width+x]&(1<<uint(y%8)) != 0 {
		return White
	}
	return Black
}

// DrawFastHLine draws a horizontal line of width w.
func (c *Canvas) DrawFastHLine(x, y, w int, col Color) {
	for i := 0; i < w; i++ {
		c.DrawPixel(x+i, y, col)
	}
}

// DrawFastVLine draws a vertical line of height h.
func (c *Canvas) DrawFastVLine(x, y, h int, col Color) {
	for i := 0; i < h; i++ {
		c.DrawPixel(x, y+i, col)
	}
}

// DrawRect draws a rectangle outline (Arduboy2 drawRect()). Like Arduboy2 it
// draws the corners twice, which matters only for Invert.
func (c *Canvas) DrawRect(x, y, w, h int, col Color) {
	c.DrawFastHLine(x, y, w, col)
	c.DrawFastHLine(x, y+h-1, w, col)
	c.DrawFastVLine(x, y, h, col)
	c.DrawFastVLine(x+w-1, y, h, col)
}

// FillRect fills a rectangle (Arduboy2 fillRect()).
func (c *Canvas) FillRect(x, y, w, h int, col Color) {
	for i := x; i < x+w; i++ {
		c.DrawFastVLine(i, y, h, col)
	}
}

// The line and circle routines below follow the algorithms in Arduboy2's
// Arduboy2.cpp (BSD-3-Clause, Copyright (c) 2016-2021 Scott Allen, derived
// from Adafruit-GFX) step for step, so ported games get the same pixels.
// Their license notices are in the NOTICE file next to this one.

// DrawLine draws a line (Arduboy2 drawLine()).
func (c *Canvas) DrawLine(x0, y0, x1, y1 int, col Color) {
	steep := abs(y1-y0) > abs(x1-x0)
	if steep {
		x0, y0 = y0, x0
		x1, y1 = y1, x1
	}
	if x0 > x1 {
		x0, x1 = x1, x0
		y0, y1 = y1, y0
	}
	dx := x1 - x0
	dy := abs(y1 - y0)
	err := dx / 2
	ystep := -1
	if y0 < y1 {
		ystep = 1
	}
	for ; x0 <= x1; x0++ {
		if steep {
			c.DrawPixel(y0, x0, col)
		} else {
			c.DrawPixel(x0, y0, col)
		}
		err -= dy
		if err < 0 {
			y0 += ystep
			err += dx
		}
	}
}

// DrawCircle draws a circle outline of radius r (Arduboy2 drawCircle()).
func (c *Canvas) DrawCircle(x0, y0, r int, col Color) {
	f := 1 - r
	ddFx, ddFy := 1, -2*r
	x, y := 0, r
	c.DrawPixel(x0, y0+r, col)
	c.DrawPixel(x0, y0-r, col)
	c.DrawPixel(x0+r, y0, col)
	c.DrawPixel(x0-r, y0, col)
	for x < y {
		if f >= 0 {
			y--
			ddFy += 2
			f += ddFy
		}
		x++
		ddFx += 2
		f += ddFx
		c.DrawPixel(x0+x, y0+y, col)
		c.DrawPixel(x0-x, y0+y, col)
		c.DrawPixel(x0+x, y0-y, col)
		c.DrawPixel(x0-x, y0-y, col)
		c.DrawPixel(x0+y, y0+x, col)
		c.DrawPixel(x0-y, y0+x, col)
		c.DrawPixel(x0+y, y0-x, col)
		c.DrawPixel(x0-y, y0-x, col)
	}
}

// FillCircle fills a circle of radius r (Arduboy2 fillCircle()).
func (c *Canvas) FillCircle(x0, y0, r int, col Color) {
	c.DrawFastVLine(x0, y0-r, 2*r+1, col)
	f := 1 - r
	ddFx, ddFy := 1, -2*r
	x, y := 0, r
	for x < y {
		if f >= 0 {
			y--
			ddFy += 2
			f += ddFy
		}
		x++
		ddFx += 2
		f += ddFx
		c.DrawFastVLine(x0+x, y0-y, 2*y+1, col)
		c.DrawFastVLine(x0+y, y0-x, 2*x+1, col)
		c.DrawFastVLine(x0-x, y0-y, 2*y+1, col)
		c.DrawFastVLine(x0-y, y0-x, 2*x+1, col)
	}
}

// Flush sends the frame to a display: lit pixels are drawn with lit, unlit
// pixels with unlit. With koebiten (black ink on a white page) pass
// lit = black and unlit = white, or pass unlit.A == 0 to draw only lit pixels
// on a display that was already cleared.
func (c *Canvas) Flush(set func(x, y int16, col color.RGBA), lit, unlit color.RGBA) {
	for page := 0; page < Height/8; page++ {
		row := c.buf[page*Width : (page+1)*Width]
		for x, b := range row {
			for bit := 0; bit < 8; bit++ {
				y := page*8 + bit
				if b&(1<<uint(bit)) != 0 {
					set(int16(x), int16(y), lit)
				} else if unlit.A != 0 {
					set(int16(x), int16(y), unlit)
				}
			}
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
