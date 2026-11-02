# C games on a console SDK → TinyGo

Written from porting 19 games of PicoLibSDK (`PicoPad/GAME/`, Miroslav Nemecek, Unlicense;
the PicoPad is an RP2040 console with a 320×240 ST7789, a d-pad and A/B/X/Y) to ESP32
boards with SPI panels and to the browser, with TinyGo 0.43 and 0.44:
github.com/sat0ken/tinygo-esp32-cyd (MIT). The compatibility package is `picopad/`, the
games are `games/<name>/`, and each comparison with the original C is in
`games/<name>/testdata/csim/`. The ports were checked on hardware by that repository's
author.

Contents: When to keep the loop · The compatibility package · Translating the game · C semantics to carry over · Comparing with the original · Assets and rights · Memory and flash.

## When to keep the loop

The original is one program on an SDK. It runs its own loop (draw, `DispUpdate`, `WaitMs`,
`KeyGet`), keeps its state in globals, and calls nothing but the SDK and libc.

- **Keep the loop** when the port must be the same game (timing, random numbers, pixels),
  when it is long (hundreds to thousands of lines per game), and when the SDK functions
  it uses are few enough to write again. Then the port can be read side by side with the
  C, and checked frame by frame against it.
- **Restructure instead** (tinygo-port §1) when the target runtime wants callbacks
  (koebiten's `Update`/`Draw`: Arduboy games), or when the original's structure is the
  problem (a sketch's `delay` scheduler).

## The compatibility package

A console type offers the SDK's functions as methods with the same names and arguments.
Each game's struct embeds it, so the calls read like the C (`g.DrawText(...)`,
`g.KeyGet()`).

| PicoLibSDK | Compatibility package (`picopad` in tinygo-esp32-cyd) |
|---|---|
| `DrawRect`, `DrawImgPal`, `DrawBlitPal`, `DrawImg4Pal`, `DrawText`, … | methods with the same names and arguments; the clipping copied from `lib_draw.c` |
| the 320×240 RGB565 frame buffer | 8-bit palette indices and a 256-colour palette (see Memory and flash) |
| `DispUpdate`, `DispDirtyRect` | send the changed rectangle in bands of 8 rows, converted to RGB565 |
| `KeyGet`, `KeyPressed`, `KeyFlush` | the SDK's key scan: read every 5 ms, repeat after 400 ms and then every 100 ms, released after 50 ms up |
| `Time`, `WaitMs` | through a `Clock` interface (`Now`, `Sleep`): the real time on the device, a fake clock in tests |
| `RandU8`, `RandU16Max`, `RandFloat`, … | the SDK's 64-bit linear congruential generator, seeded the same way |
| `img/*.cpp` arrays | Go string constants made by a generator from the C arrays (`//go:embed` files for large ones) |
| `PLAYSOUND`, `PlaySoundChan` | comments, until there is sound |
| `ResetToBootLoader` | return from the game's `Run` |

- **Buttons** come from whatever the board has: a game controller, the browser's keys, or
  a touch panel split into 3 × 3 areas. A button already held when the console starts
  (the A that chose the game in a launcher) counts only after it is released.
- **Busy waits.** C games wait with `while (KeyGet() == NOKEY) {}`. On a microcontroller
  that only burns time, but in the WebAssembly build the page never gets control back. Let
  `KeyGet`, `KeyPressed` and `Time` sleep 1 ms when the program has not slept for 20 ms.
- **Grow the package with the games.** Each game needs a few more SDK functions (2-D
  transforms in fixed point, shadows, saving a rectangle). Port each from the SDK's source
  with a test of its own; check against the original's screenshots pixel by pixel where
  the SDK's examples ship them.

## Translating the game

- One Go file per C file, the C globals as fields of the game's struct, the functions as
  methods (`DispTile` → `dispTile`), the tables as arrays or strings, `#define`s as named
  constants.
- Keep the loop. Give each game a `New(display, touch, seed)` and a `Run()` that returns
  where the C resets to the boot loader.
- Add small hooks for tests: a function that tells which keys are held, one that runs a
  single frame. They let a test drive the game frame by frame.
- Write every difference from the C in the package's doc comment: no sound, storage that
  became RAM, a shortest frame time where the C loop ran as fast as the PicoPad could
  draw.

## C semantics to carry over

- **Integer widths.** `short`, `u8`, `s16` values that wrap become `int16`, `uint8`.
  Compute in `int` and convert when storing, as C does (`x = int16(int(x) + dx)`); keep
  the wrap where the C relies on it (a free-slot marker of `-32768`, coordinates that
  overflow).
- **Time.** `u32` microseconds; a difference is taken unsigned and then compared signed:
  `(int)(Time() - t) > n` becomes `int32(g.Time()-t) > n`.
- **`float`.** Keep `float32`. C's `sinf`, `cosf`, `atan2f`, `sqrtf`, `powf` become the
  `math` functions in `float64`, converted to `float32`. A `double` constant passed to a
  `float` parameter (`RandFloatMax(PI2)`) is rounded to `float32` first.
- **Argument evaluation order.** C leaves the order unspecified. In our test,
  `f(next(), next())` ran right to left with GCC for x86-64 and left to right with clang.
  When two arguments draw random numbers (`AddMissile(..., RandS16MinMax(...),
  RandS16MinMax(...))`), write them as two statements in the order the original
  compiler used. We assumed GCC for ARM evaluates left to right and compiled the host
  harness with clang; check the original's disassembly when it matters.
- **Reused slots.** C code that fills a free entry of an object array often sets only some
  fields and relies on the others keeping old values. Copy the fields it sets, not a new
  struct, where the old values are read later.
- **Quirks and bugs.** Keep them, and say so, unless the user agrees to change them. One
  from the worked example: ZOOM waited for a short frame and counted that wait again in
  the next frame. The PicoPad never drew a frame in under 20 ms, so it never showed; in
  the browser the game ran twice as fast. The port fixes it and says so.

## Comparing with the original

1. Copy the game's C sources (strip `\r`) and its data files into a temporary directory.
2. Write a stub header in place of the SDK:
   - drawing and sound as empty macros or functions;
   - `Time()` returning a variable that the harness advances, and `WaitMs` adding to it;
   - `KeyPressed` and `KeyGet` reading the script;
   - the SDK's random generator, copied from its source;
   - the `float` functions as macros that compute in `double` and cast (`#define sinf(a)
     ((float)sin((double)(a)))`), so both sides round the same way.
3. Write a `sim.c` that runs the game's frames: copy the body of the main loop (skip
   the title and menus), print a line of the state after each frame (score, positions,
   every object), and print a CRC-32 of the lines every N frames.
4. In the Go test, run the same frames with the same script and the same seed. Hold the
   clock still within a frame and advance both clocks by the same amount between frames.
   Write the same lines into `crc32.NewIEEE()` and compare at the same frames. Use a
   scripted player that reaches every part of the game: one that steers at the nearest
   target, shoots, and changes weapon or character on a timer.
5. Keep a `run.sh` next to the test that builds and runs the harness, so the checksums
   can be made again.

What it covered in the worked example:

| Game | Compared | What it found or needed |
|---|---|---|
| PACMAN | 4,000 frames: positions, directions and states of all five, score, dots | changing one ghost's target rule makes it fail |
| VEGASLOT | 3,000 spins, with the bonus game | — |
| RAPTOR | 10,000 frames: ship, enemies, missiles, energy (level 12, every weapon) | `sinf`/`atan2f` from libm round some results the other way: from frame 5,085 the bomb was off by the last bit, until both sides computed in `double` |
| ANTS | 2,000 moves of two computer players, 21 games | — |
| PICTOR | 9,000 frames: 12 scenes, all 13 characters | the evaluation order of two random arguments |
| ZOOM | 30,000 frames of 30 ms: sectors 1 to 7 | `sinf`/`cosf` for the stars, compared exactly |
| CHESS | move counts to depth 4, and the computer's moves | — |
| TRAIN, SOKOBAN | the original's solutions for all 50 and 2,020 levels | — |

Pitfalls:

- **Drawing that changes state.** Some games advance animations or remove finished hits
  inside their display functions. Call the display functions in the harness too, with
  empty drawing underneath.
- **Time read many times per frame.** If the stub's `Time()` advanced on every call, both
  sides would have to call it the same number of times. Keep it constant within a frame.
- **Screens and menus** (title, start of a level, high scores) wait for keys and time. Run
  the frames directly in both harnesses, and test the screens separately with the fake
  clock and golden images.

## Assets and rights

PicoLibSDK's own code is Unlicense, but its games carry pictures that are not the
author's to give: characters from films, a commercial game's title logo and sprites, the
box art and levels of another game. Its TINYGAME directory is GPLv3. What the worked
example did:

- left out FLAPPY (box art and level data of a commercial game) and the GPLv3 games;
- in PICTOR, drew five film characters again at the same size and number of frames (so
  hit boxes did not change), with a small generator program, and cleared a shop's logo;
- in ZOOM (based on a 1984 arcade game, whose title and sprites it had), drew every
  sprite again, renamed the game, and replaced the original copyright lines on the title
  with instructions.

## Memory and flash

- **Frame buffer.** A 320×240 RGB565 buffer is 150 KB. On an ESP32 without PSRAM that left
  38 KB of heap; 8-bit palette indices (75 KB) left 104 KB (tinygo-best-practices §8).
  Scenes with more than 256 colours got a palette chosen ahead of time (median cut over
  the scene's pictures), set when the scene starts.
- **Images.** Strings and `//go:embed` files stay in flash; tables of slices do not
  (tinygo-best-practices §3).
- **Flash per board.** The 19 games' data came to about 4.4 MB. On the 4 MB ESP32 boards
  a menu with all of them failed to link (`.rodata` does not fit), so a build tag leaves
  the largest one out there.
- **A launcher.** When games run one after another from a menu, hand the frame buffer of
  the finished game to the next console instead of allocating a new one: the GC may not
  have freed the old 75 KB before the new allocation.
