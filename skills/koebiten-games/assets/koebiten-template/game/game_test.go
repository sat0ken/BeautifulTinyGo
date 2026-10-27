package game

import (
	"testing"

	"github.com/sago35/koebiten"
)

// Run with: tinygo test ./game   (koebiten only builds under TinyGo)

// No frame may allocate, in any scene, including frames where the HUD text
// changes. -print-allocs cannot see allocations inside runtime helpers
// (string building, conversions, append growth); testing.AllocsPerRun can.
// It needs TinyGo 0.43 or later.
func TestFramesDoNotAllocate(t *testing.T) {
	screen := koebiten.NewImage(W, H)
	for _, tc := range []struct {
		name  string
		setup func(g *Game)
		frame func(g *Game)
	}{
		{"title", func(g *Game) {}, func(g *Game) {}},
		{"play, score changing every frame", func(g *Game) { g.reset() }, func(g *Game) {
			// A block on the bottom row, away from the player, falls off
			// this tick and scores a point.
			x := int16(0)
			if g.x < W/2 {
				x = W - blockSize
			}
			g.blocks[0] = block{x: x, y: (H - 1) * 4, live: true}
		}},
		{"game over", func(g *Game) { g.reset(); g.scene = over }, func(g *Game) {}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := New()
			tc.setup(g)
			start := g.score
			n := testing.AllocsPerRun(30, func() {
				tc.frame(g)
				g.Update()
				g.Draw(screen)
			})
			if n != 0 {
				t.Fatalf("a frame allocates %.1f times, want 0", n)
			}
			if tc.name != "title" && tc.name != "game over" && g.score == start {
				t.Fatal("the score never changed, so text updates were not exercised")
			}
		})
	}
}

func TestScoreText(t *testing.T) {
	g := New()
	g.reset()
	if got := g.scoreText(); got != "SCORE 0" {
		t.Fatalf("got %q, want %q", got, "SCORE 0")
	}
	g.score = 42
	g.setScoreText()
	if got := g.scoreText(); got != "SCORE 42" {
		t.Fatalf("got %q, want %q", got, "SCORE 42")
	}
}

func TestBlockReachingPlayerEndsGame(t *testing.T) {
	g := New()
	g.reset()
	g.blocks[0] = block{x: g.x, y: (H - playerH - blockSize) * 4, live: true}
	g.frame = 1 // avoid spawning on this tick
	if err := g.Update(); err != nil {
		t.Fatal(err)
	}
	if g.scene != over {
		t.Fatalf("scene = %d, want over", g.scene)
	}
}

func TestPRNGIsDeterministic(t *testing.T) {
	a, b := New(), New()
	for i := 0; i < 100; i++ {
		if a.next() != b.next() {
			t.Fatal("two games with the same seed diverged")
		}
	}
}
