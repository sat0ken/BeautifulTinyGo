// Package game is a minimal koebiten game: dodge the falling blocks.
// Left/right move, button 0 (A, or z in the browser) starts and restarts.
//
// It keeps the patterns the koebiten-games skill asks for: integer positions,
// a deterministic PRNG kept in the game state, fixed-size arrays, and HUD text
// formatted into a fixed buffer so that no frame allocates, not even the ones
// where the score changes.
package game

import (
	"strconv"
	"unsafe"

	"github.com/sago35/koebiten"
	"tinygo.org/x/drivers/pixel"
	"tinygo.org/x/tinyfont"
)

// W is the logical screen width; see screen_*.go. H is 64 on every target.
const (
	H         = 64
	playerW   = 9
	playerH   = 4
	blockSize = 5
	maxBlocks = 8
	hudH      = 8 // the score line; blocks start below it
)

var black = pixel.NewMonochrome(0x00, 0x00, 0x00)

type scene uint8

const (
	title scene = iota
	playing
	over
)

type block struct {
	x, y int16 // y in 1/4 pixels so slow speeds stay integer
	live bool
}

// Game implements koebiten.Game.
type Game struct {
	scene    scene
	x        int16
	blocks   [maxBlocks]block
	frame    uint32
	score    uint16
	rng      uint32
	scoreBuf [16]byte // "SCORE 123"
	scoreLen int
}

// New returns a game on its title screen.
func New() *Game {
	return &Game{rng: 0x2545f491}
}

// next is xorshift32: deterministic, allocation-free and the same on every
// target, which also makes headless snapshots repeatable.
func (g *Game) next() uint32 {
	g.rng ^= g.rng << 13
	g.rng ^= g.rng >> 17
	g.rng ^= g.rng << 5
	return g.rng
}

func (g *Game) reset() {
	if g.rng == 0 {
		g.rng = 1 // xorshift never leaves zero
	}
	g.scene = playing
	g.x = W/2 - playerW/2
	g.score = 0
	g.frame = 0
	for i := range g.blocks {
		g.blocks[i].live = false
	}
	g.setScoreText()
}

// setScoreText formats the score into scoreBuf. It fits in the array, so
// append never grows it and nothing is allocated.
func (g *Game) setScoreText() {
	b := append(g.scoreBuf[:0], "SCORE "...)
	b = strconv.AppendUint(b, uint64(g.score), 10)
	g.scoreLen = len(b)
}

// scoreText views scoreBuf as a string without copying it. The string shares
// memory with the buffer: use it for drawing in this frame and never keep it.
func (g *Game) scoreText() string {
	return unsafe.String(&g.scoreBuf[0], g.scoreLen)
}

// Update advances one tick. koebiten calls it about every 32 ms.
func (g *Game) Update() error {
	switch g.scene {
	case title, over:
		if koebiten.IsKeyJustPressed(koebiten.Key0) {
			g.rng ^= g.frame | 1 // the player's timing seeds the run
			g.reset()
		}
		g.frame++
		return nil
	}
	if koebiten.IsKeyPressed(koebiten.KeyLeft) && g.x > 0 {
		g.x -= 2
	}
	if koebiten.IsKeyPressed(koebiten.KeyRight) && g.x < W-playerW {
		g.x += 2
	}
	g.frame++
	if g.frame%12 == 0 {
		g.spawn()
	}
	speed := int16(4 + g.score/5) // quarter pixels per tick
	for i := range g.blocks {
		b := &g.blocks[i]
		if !b.live {
			continue
		}
		b.y += speed
		y := b.y / 4
		if y+blockSize >= H-playerH && b.x < g.x+playerW && b.x+blockSize > g.x {
			g.scene = over
			return nil
		}
		if y >= H {
			b.live = false
			g.score++
			g.setScoreText()
		}
	}
	return nil
}

func (g *Game) spawn() {
	for i := range g.blocks {
		if !g.blocks[i].live {
			g.blocks[i] = block{x: int16(g.next() % (W - blockSize)), y: hudH * 4, live: true}
			return
		}
	}
}

// Draw renders the current frame. koebiten clears the buffer before calling
// it and sends it to the display afterwards.
func (g *Game) Draw(screen *koebiten.Image) {
	switch g.scene {
	case title:
		centered(screen, "DODGE", 26)
		centered(screen, "PRESS A", 40)
		return
	case over:
		centered(screen, "GAME OVER", 26)
		centered(screen, g.scoreText(), 36)
		centered(screen, "A: RETRY", 50)
		return
	}
	for _, b := range g.blocks {
		if b.live {
			koebiten.DrawFilledRect(screen, int(b.x), int(b.y/4), blockSize, blockSize, black)
		}
	}
	koebiten.DrawFilledRect(screen, int(g.x), H-playerH, playerW, playerH, black)
	koebiten.DrawText(screen, g.scoreText(), &tinyfont.Org01, 1, 6, black)
}

// centered draws one line of text centered horizontally; y is the baseline.
func centered(screen *koebiten.Image, s string, y int16) {
	_, w := tinyfont.LineWidth(&tinyfont.Org01, s)
	koebiten.DrawText(screen, s, &tinyfont.Org01, int16((W-int(w))/2), y, black)
}

// Layout reports the logical screen size.
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return W, H
}
