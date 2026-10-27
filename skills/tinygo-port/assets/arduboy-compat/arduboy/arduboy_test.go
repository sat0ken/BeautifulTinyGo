package arduboy

import (
	"image/color"
	"testing"
)

type px struct{ x, y int }

func lit(c *Canvas) map[px]bool {
	m := map[px]bool{}
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			if c.GetPixel(x, y) == White {
				m[px{x, y}] = true
			}
		}
	}
	return m
}

func expectLit(t *testing.T, c *Canvas, want ...px) {
	t.Helper()
	got := lit(c)
	if len(got) != len(want) {
		t.Fatalf("lit pixels = %v, want %v", got, want)
	}
	for _, p := range want {
		if !got[p] {
			t.Fatalf("pixel %v not lit; lit = %v", p, got)
		}
	}
}

func TestBufferLayoutMatchesArduboy(t *testing.T) {
	var c Canvas
	c.DrawPixel(3, 10, White)
	if c.Buffer()[1*Width+3] != 1<<2 {
		t.Fatalf("byte = %08b, want bit 2 of page 1, column 3", c.Buffer()[1*Width+3])
	}
	c.DrawPixel(3, 10, Invert)
	if c.GetPixel(3, 10) != Black {
		t.Fatal("Invert did not clear the pixel")
	}
	c.DrawPixel(-1, 0, White)
	c.DrawPixel(0, Height, White)
	expectLit(t, &c)
}

func TestDrawBitmapVerticalBytes(t *testing.T) {
	var c Canvas
	c.DrawBitmap(0, 0, []byte{0x01, 0x80}, 2, 8, White)
	expectLit(t, &c, px{0, 0}, px{1, 7})
}

func TestDrawSlowXYBitmapHorizontalBytes(t *testing.T) {
	var c Canvas
	// 10 pixels wide: two bytes per row, MSB first.
	c.DrawSlowXYBitmap(0, 0, []byte{0x80, 0x40}, 10, 1, White)
	expectLit(t, &c, px{0, 0}, px{9, 0})
}

// A 2x8 sprite with two frames.
var sprite = []byte{2, 8,
	0x01, 0x00, // frame 0
	0x00, 0x02, // frame 1
}

func TestSelfMaskedUsesFrame(t *testing.T) {
	var c Canvas
	c.DrawSelfMasked(5, 5, sprite, 1)
	expectLit(t, &c, px{6, 6})
}

func TestOverwriteClearsZeroBits(t *testing.T) {
	var c Canvas
	c.FillScreen(White)
	c.DrawOverwrite(0, 0, sprite, 0)
	if c.GetPixel(0, 0) != White || c.GetPixel(0, 1) != Black || c.GetPixel(1, 0) != Black {
		t.Fatal("overwrite did not copy the frame exactly")
	}
	if c.GetPixel(2, 0) != White || c.GetPixel(0, 8) != White {
		t.Fatal("overwrite touched pixels outside the sprite")
	}
}

func TestEraseClearsSetBits(t *testing.T) {
	var c Canvas
	c.FillScreen(White)
	c.DrawErase(0, 0, []byte{1, 8, 0x01}, 0)
	if c.GetPixel(0, 0) != Black || c.GetPixel(0, 1) != White {
		t.Fatal("erase should clear only set bits")
	}
}

func TestPlusMaskAndExternalMask(t *testing.T) {
	for _, tc := range []struct {
		name string
		draw func(c *Canvas)
	}{
		{"plus", func(c *Canvas) { c.DrawPlusMask(0, 0, []byte{1, 8, 0x01, 0x03}, 0) }},
		{"external", func(c *Canvas) { c.DrawExternalMask(0, 0, []byte{1, 8, 0x01}, []byte{0x03}, 0, 0) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var c Canvas
			c.FillScreen(White)
			tc.draw(&c)
			// bit 0: mask 1, image 1 -> White; bit 1: mask 1, image 0 -> Black;
			// bit 2: mask 0 -> unchanged (White).
			if c.GetPixel(0, 0) != White || c.GetPixel(0, 1) != Black || c.GetPixel(0, 2) != White {
				t.Fatalf("got %v %v %v", c.GetPixel(0, 0), c.GetPixel(0, 1), c.GetPixel(0, 2))
			}
		})
	}
}

func TestLineMatchesArduboyBresenham(t *testing.T) {
	var c Canvas
	c.DrawLine(0, 0, 3, 1, White)
	expectLit(t, &c, px{0, 0}, px{1, 0}, px{2, 1}, px{3, 1})
}

func TestFillCircleRadiusOne(t *testing.T) {
	var c Canvas
	c.FillCircle(5, 5, 1, White)
	expectLit(t, &c, px{5, 4}, px{5, 5}, px{5, 6}, px{4, 5}, px{6, 5})
}

func TestCollide(t *testing.T) {
	a := Rect{0, 0, 4, 4}
	if !Collide(a, Rect{3, 3, 2, 2}) || Collide(a, Rect{4, 0, 2, 2}) {
		t.Fatal("rect collide does not match Arduboy2 edges")
	}
	if !CollidePoint(Point{3, 3}, a) || CollidePoint(Point{4, 3}, a) {
		t.Fatal("point collide does not match Arduboy2 edges")
	}
}

func TestFlushOnlyLitWhenUnlitTransparent(t *testing.T) {
	var c Canvas
	c.DrawPixel(1, 2, White)
	n := 0
	c.Flush(func(x, y int16, col color.RGBA) {
		n++
		if x != 1 || y != 2 {
			t.Fatalf("unexpected pixel %d,%d", x, y)
		}
	}, color.RGBA{A: 255}, color.RGBA{})
	if n != 1 {
		t.Fatalf("set called %d times, want 1", n)
	}
}

func TestDrawingDoesNotAllocate(t *testing.T) {
	var c Canvas
	plus := []byte{8, 8, 0x3c, 0x7e, 0x7e, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7e, 0x7e, 0x3c, 0x3c, 0x7e}
	set := func(x, y int16, col color.RGBA) {}
	n := testing.AllocsPerRun(20, func() {
		c.Clear()
		c.DrawPlusMask(10, 10, plus, 0)
		c.DrawSelfMasked(30, 3, sprite, 1)
		c.FillCircle(64, 32, 10, White)
		c.DrawLine(0, 63, 127, 0, Invert)
		c.Flush(set, color.RGBA{A: 255}, color.RGBA{})
	})
	if n != 0 {
		t.Fatalf("allocations per frame = %v, want 0", n)
	}
}
