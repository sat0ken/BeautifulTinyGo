//go:build tinygo && wasm

// Package preview draws a TinyGo display program on an HTML canvas, so it can
// be checked in a browser, or headlessly with tools/capture.cjs, before it is
// flashed to a board.
//
// Copy this directory into your module (for example as ./preview) and only
// import it from files guarded by "//go:build tinygo && wasm".
package preview

import (
	"image/color"
	"syscall/js"
)

// Display is an RGBA framebuffer that satisfies tinygo.org/x/drivers.Displayer.
//
// SetPixel and FillRectangle only touch Go memory. Display copies the whole
// frame to JavaScript in one call, so call it once per frame, exactly as you
// would for a buffered display such as the SSD1306.
type Display struct {
	w, h  int16
	buf   []byte   // RGBA, w*h*4 bytes
	jsBuf js.Value // Uint8Array returned by previewInit
	show  js.Value // window.display
}

// NewDisplay asks the page (previewInit) for a canvas of the given size and
// returns a display that draws to it.
func NewDisplay(width, height int16) *Display {
	g := js.Global()
	d := &Display{
		w:   width,
		h:   height,
		buf: make([]byte, int(width)*int(height)*4),
	}
	d.jsBuf = g.Call("previewInit", int(width), int(height))
	d.show = g.Get("display")
	return d
}

// Size implements drivers.Displayer.
func (d *Display) Size() (x, y int16) {
	return d.w, d.h
}

// SetPixel implements drivers.Displayer. Out-of-range pixels are ignored.
func (d *Display) SetPixel(x, y int16, c color.RGBA) {
	if x < 0 || y < 0 || x >= d.w || y >= d.h {
		return
	}
	i := (int(y)*int(d.w) + int(x)) * 4
	d.buf[i] = c.R
	d.buf[i+1] = c.G
	d.buf[i+2] = c.B
	d.buf[i+3] = 0xff
}

// FillRectangle has the same signature as the st7789, ili9341 and ssd1306
// drivers, so application code can depend on one small interface for both.
func (d *Display) FillRectangle(x, y, width, height int16, c color.RGBA) error {
	for j := y; j < y+height; j++ {
		for i := x; i < x+width; i++ {
			d.SetPixel(i, j, c)
		}
	}
	return nil
}

// FillScreen fills the whole frame with one color.
func (d *Display) FillScreen(c color.RGBA) {
	for i := 0; i < len(d.buf); i += 4 {
		d.buf[i] = c.R
		d.buf[i+1] = c.G
		d.buf[i+2] = c.B
		d.buf[i+3] = 0xff
	}
}

// Display implements drivers.Displayer: it sends the frame to the page.
func (d *Display) Display() error {
	js.CopyBytesToJS(d.jsBuf, d.buf)
	d.show.Invoke()
	return nil
}
