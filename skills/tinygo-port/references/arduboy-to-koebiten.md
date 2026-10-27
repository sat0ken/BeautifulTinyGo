# Arduboy → TinyGo (koebiten) mapping

Checked against Arduboy2 6.0.0 (`Arduboy2.h`, `Arduboy2Core.h`, `Sprites.h`; BSD-3-Clause,
Scott Allen), ArduboyTones 1.0.3 (MIT, Scott Allen), koebiten v0.6.0 and TinyGo 0.43.

The Arduboy is an ATmega32U4 with 2.5 KB RAM, a 128×64 SSD1306 OLED, a d-pad, A and B,
and a piezo speaker. Running TinyGo on the Arduboy itself is not the goal: the port runs
on koebiten hardware (same screen size and key layout) and in the browser.

## The compatibility package

`assets/arduboy-compat/arduboy` (copy it into the project) gives an `arduboy.Canvas`
with Arduboy2's frame-buffer layout and drawing semantics, tested with `go test` and
`tinygo test`, and allocation-free per frame:

| Arduboy2 | `arduboy.Canvas` |
|---|---|
| `clear()`, `fillScreen(c)` | `Clear()`, `FillScreen(c)` |
| `drawPixel`, `getPixel` | `DrawPixel`, `GetPixel` |
| `drawFastHLine`, `drawFastVLine`, `drawRect`, `fillRect` | same names, `int` arguments |
| `drawLine`, `drawCircle`, `fillCircle` | same names; same algorithms, so the same pixels |
| `drawBitmap(x, y, bmp, w, h, c)` | `DrawBitmap(x, y, bmp, w, h, c)` |
| `drawSlowXYBitmap(...)` | `DrawSlowXYBitmap(...)` |
| `Sprites::drawOverwrite/SelfMasked/Erase/PlusMask(x, y, s, frame)` | `DrawOverwrite`, `DrawSelfMasked`, `DrawErase`, `DrawPlusMask` |
| `Sprites::drawExternalMask(x, y, s, m, f, mf)` | `DrawExternalMask(x, y, s, m, f, mf)` |
| `WHITE`, `BLACK`, `INVERT` | `arduboy.White`, `arduboy.Black`, `arduboy.Invert` |
| `getBuffer()` / `sBuffer` | `Buffer()` (same 8 pages × 128 bytes) |
| `collide(Rect, Rect)`, `collide(Point, Rect)` | `arduboy.Collide`, `arduboy.CollidePoint` |
| `display()` | `canvas.Flush(set, lit, unlit)` once at the end of koebiten's `Draw` |

Not included: `drawRoundRect`, `fillRoundRect`, `drawTriangle`, `fillTriangle`,
`drawCompressed` (Cabi format) and text. Port those when a game needs them, following
Arduboy2's source, and add a test.

Flush to koebiten's display directly. koebiten's `Draw*` helpers check the destination
with reflection on every call, which is far too slow per pixel:

```go
koebiten.SetHardware(hardware.Device)
set := hardware.Device.GetDisplay().SetPixel
...
// in Draw:
g.canvas.Flush(set, ink.RGBA(), color.RGBA{}) // koebiten cleared to white; send lit pixels only
```

Arduboy "white" is a lit OLED pixel; koebiten draws black ink on a white page. Mapping lit
pixels to koebiten's ink keeps every game readable, with light and dark swapped compared
with the original.

`assets/arduboy-compat/example/` is a complete koebiten program built this way (WebAssembly
and zero-kb02 builds verified).

## Bitmaps and sprites

- Copy PROGMEM arrays as Go byte slices: `const unsigned char PROGMEM ship[] = {16, 8, ...};`
  → `var ship = []byte{16, 8, ...}`. Keep the order and the header bytes exactly.
- Formats (from Arduboy2's documentation):
  - `drawBitmap`: no header; each byte is a vertical strip of 8 pixels, bit 0 on top;
    rows of `w` bytes; `h` a multiple of 8. Set bits get the colour, clear bits are left.
  - `drawSlowXYBitmap`: no header; horizontal bytes, most significant bit on the left,
    rows padded to whole bytes.
  - `Sprites`: two header bytes (width, height), then each frame's vertical bytes, frame
    after frame. `drawPlusMask` interleaves image and mask bytes (image first), so each
    row is twice the width. `drawExternalMask` takes a mask array without the header.
- Do not convert sprites to PNG for koebiten's `NewImageFromFS` unless you need its
  `GeoM` scaling: koebiten images can only draw ink, so `drawErase`, masks and `INVERT`
  would be lost.

## Game loop and timing

| Arduboy2 | koebiten port |
|---|---|
| `arduboy.begin()`, `setFrameRate(60)` | `koebiten.SetHardware(hardware.Device)`; koebiten ticks every 32 ms (≈31 FPS), fixed. Simplest faithful port: call the original per-frame logic twice in each `Update`, so every value below keeps its 60 FPS meaning |
| `if (!arduboy.nextFrame()) return;` | not needed: `Update` and `Draw` are called once per tick |
| `arduboy.pollButtons()` | not needed: koebiten updates key state before `Update` |
| `arduboy.clear()` … `arduboy.display()` | `canvas.Clear()` at the start of `Draw`, `canvas.Flush(...)` at the end |
| `everyXFrames(n)` | keep a frame counter; when rescaling instead of running two logic frames per tick, use `n/2` (round, and note it) |
| `frameCount` | your own `uint32` counter in the game state |
| movement in pixels per frame | doubled per tick, or kept in fixed point (`x16 += vx * 2`) |
| `initRandomSeed()`, `random(a, b)` | xorshift in the game state, seeded from the frame count when the player starts |

Keep game logic in `Update` and drawing in `Draw` even when the original mixes them in
`loop()`; draw from state, not while updating it.

## Buttons

Arduboy button bits (`Arduboy2Core.h`): `UP_BUTTON` 1<<7, `DOWN_BUTTON` 1<<4,
`LEFT_BUTTON` 1<<5, `RIGHT_BUTTON` 1<<6, `A_BUTTON` 1<<3, `B_BUTTON` 1<<2.

| Arduboy2 | koebiten |
|---|---|
| `pressed(LEFT_BUTTON)` | `koebiten.IsKeyPressed(koebiten.KeyLeft)` (Up/Down/Right likewise) |
| `pressed(A_BUTTON)` / `B_BUTTON` | `koebiten.IsKeyPressed(koebiten.Key0)` / `Key1` |
| `pressed(A_BUTTON | B_BUTTON)` (all held) | both keys pressed |
| `anyPressed(mask)`, `notPressed(mask)` | any / none of the keys pressed |
| `justPressed(b)`, `justReleased(b)` | `koebiten.IsKeyJustPressed`, `IsKeyJustReleased` |

The example's `pressed`/`justPressed` helpers keep the original button expressions
readable. In the browser A is `z` (or Space/Enter) and B is `x`.

## Text

Arduboy2's 5×7 font (`print`, `setCursor`, `setTextSize`) is not part of the package.
Draw text after `Flush` with `koebiten.DrawText(screen, s, &tinyfont.Org01, x, y+6, ink)`
(Arduboy's cursor is the top-left corner; tinyfont's `y` is the baseline). Build numeric
strings only when the number changes. Layout may need small adjustments; note them.

## Sound

`ArduboyTones` (`tone(f, d)`, `tones(seq)` with `TONES_END`/`TONES_REPEAT`) and
`Arduboy2Audio` (`audio.on()`, `audio.enabled()`) have no koebiten equivalent. Options:

- boards with a buzzer pin: `tinygo.org/x/drivers/tone` driven from `Update` with a small
  sequencer that steps through the original `uint16` arrays: (frequency, duration in ms)
  pairs, a frequency of exactly `0x8000` (`TONES_END`) ends the sequence, `0x8001`
  (`TONES_REPEAT`) restarts it, and `0x8000` added to a real frequency
  (`TONE_HIGH_VOLUME`) means high volume. Mask it off before using the frequency;
- otherwise stub the sound calls and list them in `PORTING.md`.

## Saving

`EEPROM.put/get` at `EEPROM_STORAGE_SPACE_START`: koebiten has no storage API. Use
`machine.Flash` on chips that have it (RP2040 does), keeping a version byte and a checksum,
or keep scores in RAM and say so.

## Licenses

Arduboy games are released under many licenses, and some under none.

- Keep the game's copyright notice and license file in the port, and credit the author.
- GPL games stay GPL: the port must be released under the GPL too.
- No license means no permission to redistribute. Tell the user before porting; a private
  port for personal use, or asking the author, are the options.
- Arduboy2 itself is BSD-3-Clause; the compatibility package's line and circle routines
  follow its algorithms and carry that notice.
