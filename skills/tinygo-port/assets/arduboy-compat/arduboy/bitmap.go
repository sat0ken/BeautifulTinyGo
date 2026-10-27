package arduboy

// DrawBitmap draws an Arduboy2 drawBitmap() image: each byte is a vertical
// column of 8 pixels with bit 0 at the top, rows of w bytes, h a multiple of
// 8. Set bits are drawn in col (White, Black or Invert); clear bits leave the
// screen unchanged. The array has no width/height header.
func (c *Canvas) DrawBitmap(x, y int, bitmap []byte, w, h int, col Color) {
	pages := (h + 7) / 8
	for p := 0; p < pages; p++ {
		for i := 0; i < w; i++ {
			k := p*w + i
			if k >= len(bitmap) {
				return
			}
			b := bitmap[k]
			for bit := 0; bit < 8; bit++ {
				if b&(1<<uint(bit)) != 0 {
					c.DrawPixel(x+i, y+p*8+bit, col)
				}
			}
		}
	}
}

// DrawSlowXYBitmap draws an Arduboy2 drawSlowXYBitmap() image: horizontal
// bytes, most significant bit on the left, each row padded to whole bytes.
// Set bits are drawn in col; clear bits leave the screen unchanged.
func (c *Canvas) DrawSlowXYBitmap(x, y int, bitmap []byte, w, h int, col Color) {
	stride := (w + 7) / 8
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			k := j*stride + i/8
			if k >= len(bitmap) {
				return
			}
			if bitmap[k]&(0x80>>uint(i%8)) != 0 {
				c.DrawPixel(x+i, y+j, col)
			}
		}
	}
}

// Sprites mode, matching the Arduboy2 Sprites functions.
type spriteMode uint8

const (
	modeOverwrite spriteMode = iota
	modeSelfMasked
	modeErase
	modePlusMask
	modeExternalMask
)

// SpriteSize returns the width and height stored in the first two bytes of an
// Arduboy2 Sprites array.
func SpriteSize(sprite []byte) (w, h int) {
	if len(sprite) < 2 {
		return 0, 0
	}
	return int(sprite[0]), int(sprite[1])
}

// DrawOverwrite is Sprites::drawOverwrite(): every pixel in the sprite
// rectangle takes the image value (1 = White, 0 = Black).
func (c *Canvas) DrawOverwrite(x, y int, sprite []byte, frame int) {
	c.drawSprite(x, y, sprite, nil, frame, 0, modeOverwrite)
}

// DrawSelfMasked is Sprites::drawSelfMasked(): set bits become White, clear
// bits leave the screen unchanged.
func (c *Canvas) DrawSelfMasked(x, y int, sprite []byte, frame int) {
	c.drawSprite(x, y, sprite, nil, frame, 0, modeSelfMasked)
}

// DrawErase is Sprites::drawErase(): set bits become Black, clear bits leave
// the screen unchanged.
func (c *Canvas) DrawErase(x, y int, sprite []byte, frame int) {
	c.drawSprite(x, y, sprite, nil, frame, 0, modeErase)
}

// DrawPlusMask is Sprites::drawPlusMask(): after the width/height header,
// bytes come in (image, mask) pairs. Where the mask bit is 1 the pixel takes
// the image bit; where it is 0 the screen is unchanged.
func (c *Canvas) DrawPlusMask(x, y int, sprite []byte, frame int) {
	c.drawSprite(x, y, sprite, nil, frame, 0, modePlusMask)
}

// DrawExternalMask is Sprites::drawExternalMask(): like DrawPlusMask with the
// mask in a separate array that has no width/height header.
func (c *Canvas) DrawExternalMask(x, y int, sprite, mask []byte, frame, maskFrame int) {
	c.drawSprite(x, y, sprite, mask, frame, maskFrame, modeExternalMask)
}

func (c *Canvas) drawSprite(x, y int, sprite, mask []byte, frame, maskFrame int, mode spriteMode) {
	w, h := SpriteSize(sprite)
	if w == 0 || h == 0 {
		return
	}
	pages := (h + 7) / 8
	frameSize := w * pages
	data := sprite[2:]
	step := 1
	if mode == modePlusMask {
		step = 2
	}
	base := frame * frameSize * step
	mbase := maskFrame * frameSize
	for p := 0; p < pages; p++ {
		for i := 0; i < w; i++ {
			k := base + (p*w+i)*step
			if k >= len(data) {
				return
			}
			img := data[k]
			m := byte(0xff)
			switch mode {
			case modePlusMask:
				if k+1 >= len(data) {
					return
				}
				m = data[k+1]
			case modeExternalMask:
				mk := mbase + p*w + i
				if mk >= len(mask) {
					return
				}
				m = mask[mk]
			}
			// Like Arduboy2, whole bytes are drawn even when h is not a
			// multiple of 8; the padding bits come from the array.
			for bit := 0; bit < 8; bit++ {
				px, py := x+i, y+p*8+bit
				on := img&(1<<uint(bit)) != 0
				switch mode {
				case modeOverwrite:
					if on {
						c.DrawPixel(px, py, White)
					} else {
						c.DrawPixel(px, py, Black)
					}
				case modeSelfMasked:
					if on {
						c.DrawPixel(px, py, White)
					}
				case modeErase:
					if on {
						c.DrawPixel(px, py, Black)
					}
				case modePlusMask, modeExternalMask:
					if m&(1<<uint(bit)) != 0 {
						if on {
							c.DrawPixel(px, py, White)
						} else {
							c.DrawPixel(px, py, Black)
						}
					}
				}
			}
		}
	}
}

// Rect and Point mirror Arduboy2's types for collide().
type Rect struct{ X, Y, W, H int }

// Point is an Arduboy2 Point.
type Point struct{ X, Y int }

// Collide reports whether two rectangles overlap (Arduboy2 collide(Rect, Rect)).
func Collide(a, b Rect) bool {
	return !(b.X >= a.X+a.W || b.X+b.W <= a.X || b.Y >= a.Y+a.H || b.Y+b.H <= a.Y)
}

// CollidePoint reports whether p is inside r (Arduboy2 collide(Point, Rect)).
func CollidePoint(p Point, r Rect) bool {
	return p.X >= r.X && p.X < r.X+r.W && p.Y >= r.Y && p.Y < r.Y+r.H
}
