---
name: koebiten-games
description: >-
  Make small games with koebiten (github.com/sago35/koebiten), the Ebitengine/miniten-style
  game package for TinyGo, for 128x64 one-bit screens: zero-kb02 and other TinyGo Keeb
  keyboards, gopher-board, Gopher Badge, PyBadge/GoBadge, Wio Terminal, MacroPad RP2040,
  and the browser through WebAssembly. Use this whenever the user mentions koebiten, wants
  a mini game, demo or toy on a TinyGo board or keyboard display, asks for an
  "Ebitengine-like" game on a microcontroller, or wants to add a board to koebiten. Pairs
  with tinygo-best-practices (general TinyGo rules) and tinygo-port (Arduboy games to
  koebiten).
---

# koebiten games

koebiten is sago35's small game package for TinyGo (MIT license). It began from
eihigh's miniten and borrows names from Ebitengine, but it is not Ebitengine: many
Ebitengine APIs do not exist, and its README says the API is not fixed yet. Everything
below was checked against **koebiten v0.6.0** (July 2026) with TinyGo 0.43.

**Before using any koebiten API, open the version the project actually uses**:
`go list -m -f '{{.Dir}}' github.com/sago35/koebiten` gives the module directory; read
`koebiten.go`, `game.go`, `image.go`, `inpututil.go` and the `hardware/` file for the
board. Do not write an Ebitengine call from memory and hope it exists, and say in your
report which koebiten files you read.

## 1. Program shape

```go
func main() {
	koebiten.SetHardware(hardware.Device) // chosen by build tags, including wasm
	koebiten.SetWindowSize(game.W, game.H) // kept for Ebitengine familiarity; does nothing
	koebiten.SetWindowTitle("Dodge")       // likewise
	if err := koebiten.RunGame(game.New()); err != nil {
		println(err.Error())
	}
}
```

- A game implements `Update() error`, `Draw(screen *koebiten.Image)` and
  `Layout(w, h int) (int, int)`. Put it in its own package (`game/`) and keep `main` to
  the lines above.
- `koebiten.Run(func())` is a shortcut for a single function called every tick.
- Returning `koebiten.Termination` from `Update` makes `RunGame` return `nil`; any other
  error stops the loop and is returned. A menu that launches several games (see
  koebiten's `games/all`) is built on this.
- `koebiten.SetRotation(mode)` rotates the logical screen for boards mounted sideways.

## 2. The frame loop

- `RunGame` ticks every **32 ms (about 31 frames per second)**. The doc comments copied
  from Ebitengine mention 60 TPS; the code does not do that. A port from a 60 FPS game
  must double per-tick speeds or halve timer counts (§7 of tinygo-port).
- Each tick: read keys, clear the buffer to white, call `Update`, call `Draw`, send the
  frame. `Draw` therefore redraws everything every frame.
- `Draw` receives a nil `*Image`: pass `screen` (or `nil`) to the drawing functions and
  they draw on the hardware display. Do not call methods on `screen`.
- Keep game logic in `Update` and only rendering in `Draw`. Count time in ticks
  (31 ticks ≈ 1 s) and move things in integers; use sub-pixel integers (for example
  quarter pixels) for slow speeds instead of floats.
- `Update` plus `Draw` must fit in 32 ms on the slowest target. Build with
  `-tags koebiten_benchmark` to print, every 32 frames, how much of the 32 ms budget each
  frame used (in percent); values near 100 mean dropped frames.

## 3. Screen, colours and per-board size

- The logical screen is **128×64, one bit per pixel**, black ink on a white background
  (koebiten clears to white even on SSD1306 OLEDs). Draw with
  `black := pixel.NewMonochrome(0, 0, 0)`.
- On colour panels koebiten scales the screen 2× and centres it (Gopher Badge, PyBadge,
  Wio Terminal). **On gopher-board (ST7789 240×240) the screen is 100×64** in v0.6.0:
  pixels at x ≥ 100 are dropped. If gopher-board is a target, keep the width in a constant
  chosen by build tag (`//go:build gopher_board_spi`), as the template does, and lay out
  text from it. The browser build is always 128×64, so the preview will not show this
  cropping.
- `Draw` has no way to ask the size, so the layout constants are the source of truth.

## 4. Drawing

- Text: `koebiten.DrawText(screen, s, &tinyfont.Org01, x, y, black)`; `y` is the baseline.
  `tinyfont.LineWidth(&tinyfont.Org01, s)` returns `(inner, outer)` widths for centring.
  Org01 is the small font every koebiten game uses; larger tinyfont fonts cost flash.
- Shapes take `int` coordinates: `DrawRect`, `DrawFilledRect`, `DrawLine`, `DrawCircle`,
  `DrawFilledCircle`, `DrawTriangle`, `DrawFilledTriangle`.
- Images: embed PNGs (`//go:embed *.png`, `var fsys embed.FS`), load each one once with
  `koebiten.NewImageFromFS(fsys, "gopher.png")` (it panics on error, so do it at start-up),
  and draw with `img.DrawImage(screen, op)` where `op.GeoM.Translate(float32(x), float32(y))`.
  Dark pixels in the PNG are ink; light pixels are left untouched. `Scale` and `Rotate`
  in the `GeoM` work but transform every pixel with float math, which is slow on RP2040.
- `DrawImageFS`/`DrawImageFSWithOptions` are deprecated; do not use them in new code.
- `koebiten.Println(...)` writes debug lines at the top of the screen. It uses `fmt` and
  allocates every call: fine for a quick check, not for the game's HUD.
- `koebiten.NewImage(w, h)` is an off-screen one-bit image (useful as a test screen).

## 5. Input

- `koebiten.IsKeyPressed(k)`, `IsKeyJustPressed(k)`, `IsKeyJustReleased(k)`,
  `KeyPressDuration(k)` (ticks held).
- Keys: `Key0`–`Key11` (switches), `KeyLeft/Right/Up/Down` (also `KeyArrow*`),
  `KeyRotaryLeft/Right/Button`, `KeyJoystick`.
- What exists depends on the board; read its `hardware/*.go`:
  - gopher-board: `Key0` = A, `Key1` = B, plus the d-pad;
  - zero-kb02: twelve switches, a rotary encoder and a joystick;
  - browser: `z`/`Space`/`Enter` = `Key0`, `x` = `Key1`, `c` = `Key2`, `v` = `Key3`,
    arrows (or `e`/`d`/`s`/`f`, `h`/`j`/`k`/`l`) = d-pad.
- Design for the common subset (d-pad, `Key0`, `Key1`) unless the user targets one
  board, and tell the user which physical keys do what on each target.

## 6. Memory and speed rules

- RAM is the limit before flash: a koebiten game that runs on RP2040 can fail to fit a
  32 KB SAMD21 (flappygopher does not fit a SAMD21 XIAO). Prefer RP2040-class boards.
- **No allocation in any frame**, including the frames where the score or timer changes.
  koebiten's own sample games build HUD text with `"Score: "+strconv.Itoa(n)` inside
  `Draw`, which allocates every frame; do not copy that, and do not settle for "only when
  the value changes" either. Format into a fixed array in the game state and draw a view
  of it, as the template does:
  ```go
  b := append(g.scoreBuf[:0], "SCORE "...)        // [16]byte field: never grows
  b = strconv.AppendUint(b, uint64(g.score), 10)
  g.scoreLen = len(b)
  ...
  koebiten.DrawText(screen, unsafe.String(&g.scoreBuf[0], g.scoreLen), &tinyfont.Org01, 1, 6, black)
  ```
  The string shares memory with the array, which is safe because it is drawn at once and
  never kept. Constant labels (`"GAME OVER"`) and slices of constant strings are free.
- Entities in fixed-size arrays with a `live` flag; no maps or `append` in `Update`.
- Keep a deterministic PRNG (xorshift) in the game state. It allocates nothing, behaves
  the same on every target, and makes headless snapshots repeatable. Seed it from the
  tick count when the player presses start.
- If you need floats, use `float32` and `github.com/chewxy/math32` (koebiten already
  depends on it), never `float64` math in the loop.

## 7. Checking a game

The template (`assets/koebiten-template/`) has all of this in a Makefile.

1. **Host tests:** `tinygo test ./game` (koebiten only builds under TinyGo). Include a
   `testing.AllocsPerRun` test that runs `Update` and `Draw` on `koebiten.NewImage(W, H)`
   and expects 0 **in every scene** (title, play, end), and in play arrange the state so
   the score or timer text changes during the measured frames (assert that it did).
   `-print-allocs` cannot see string building; a test that only covers the title screen
   proves nothing. The template's `TestFramesDoNotAllocate` shows how.
2. **Browser build:** `tinygo build -o static/main.wasm -target=wasm -no-debug -panic=trap .`,
   then copy `index.html`, `main.js`, `screen-emulator.js` from the koebiten module's
   `static/` and `wasm_exec.js` from `$(tinygo env TINYGOROOT)/targets/`.
3. **Headless snapshots:** `node tools/capture.cjs static/main.wasm --duration 3000
   --at 200,1500,2500 --keys "300:z:80,1200:ArrowLeft:400"`. Read the PNGs: title screen,
   play, game over. Overlaps (for example sprites over the score line) are easy to spot here.
   Script the keys so that every scene is reached (lose on purpose, or shorten the round
   with a test-only constant), and name in your report which snapshot shows which scene.
4. **Re-check after the last change.** Any edit after a capture makes that capture stale.
   Rebuild the wasm, re-run the capture and look at every scene again before you report;
   never describe screens from an earlier build.
5. **Human play:** `python3 -m http.server 8000 --directory static` and open
   http://localhost:8000.
6. **Firmware:** koebiten's custom targets live in its module:
   `tinygo build -size short -target "$(go list -m -f '{{.Dir}}' github.com/sago35/koebiten)/targets/zero-kb02.json" .`
   (also `gopher-board-spi.json`, `gopher-board-i2c.json`, `conf2025badge.json`); built-in
   targets `gopher-badge`, `pybadge`, `wioterminal`, `macropad-rp2040`. Report flash and RAM.

## 8. Adding a board

`koebiten.SetHardware` takes anything that implements
`koebiten.Hardware { Init() error; GetDisplay() koebiten.Displayer; KeyUpdate() error }`,
where `koebiten.Displayer` is `drivers.Displayer` plus `ClearDisplay()` and
`ClearBuffer()`. `KeyUpdate` reports keys with `koebiten.AppendPressedKeys`,
`AppendJustPressedKeys` and `AppendJustReleasedKeys`. Follow an existing file in
`hardware/` (debouncing, white background), give non-standard hardware a build tag, and
read koebiten PRs #7 and #8 before sending one upstream. For a one-off board, write the
type in your own module and pass it to `SetHardware` instead of `hardware.Device`.

## 9. Checklist

- [ ] koebiten version pinned in `go.mod`; APIs checked in that version's source.
- [ ] `Update` holds logic, `Draw` only draws; speeds and timers assume 32 ms ticks.
- [ ] Layout uses width constants per board (100 on gopher-board); text centred from them.
- [ ] Images loaded once at start-up; no `Println`, string building, maps or `append` per frame;
      HUD text drawn from a fixed buffer.
- [ ] `tinygo test ./game` passes, with `AllocsPerRun` == 0 in every scene and while the
      score/timer text changes.
- [ ] Snapshots of every scene taken from the final build and looked at; `make serve` explained.
- [ ] Firmware built for every board the user named; sizes reported; key mapping per
      board written down.
