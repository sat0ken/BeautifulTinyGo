# WebAssembly preview for display programs

The template lives in `assets/wasm-preview/` next to this skill. It was built and run
with TinyGo 0.43.0, tinygo.org/x/drivers 0.36.0 and tinyfont 0.7.0: the example builds
for `-target=wasm` and `-target=waveshare-rp2040-zero`, type-checks for `-target=m5stack`,
and `tools/capture.cjs` produces correct snapshots for it and for koebiten 0.6.0 games.

## What is in the template

```
wasm-preview/
├── go.mod                 module wasmpreview (rename when you copy)
├── Makefile               wasm / capture / serve / hw
├── preview/
│   ├── display.go         Display: drivers.Displayer + FillRectangle + FillScreen
│   └── input.go           previewKey hook, Pressed(key), Button(keys...) func() bool
├── web/
│   ├── index.html         canvas page (integer scaling, light/dark)
│   └── preview.js         previewInit, display, keyboard → previewKey
├── tools/
│   └── capture.cjs        headless runner: scripted keys → PNG snapshots + JSON summary
└── example/
    ├── main.go            three-reel toy: logic + drawing, no machine import
    ├── hw_wasm.go         //go:build tinygo && wasm
    ├── hw_m5stack.go      //go:build m5stack           (ILI9341 320x240, buttons A/B/C)
    └── hw_gopherboard.go  //go:build waveshare_rp2040_zero (ST7789 240x240, d-pad)
```

Everything in `preview/` is guarded by `//go:build tinygo && wasm`, so it never ends up
in firmware.

## The JavaScript contract

| Name | Defined by | Meaning |
|---|---|---|
| `previewInit(w, h)` | page / capture | Called once by `preview.NewDisplay`; returns a `Uint8Array` of `w*h*4` RGBA bytes. |
| `display()` | page / capture | Called by `Display()` after `js.CopyBytesToJS` copied the frame. |
| `previewKey(key, down)` | Go (`preview` package) | The page calls it on every keydown/keyup with `KeyboardEvent.key`. |
| `screenBuffer` | page / capture | The current frame buffer. koebiten writes here directly (128×64). |
| `wasmKeyEvent(key)` | Go (koebiten) | koebiten's input hook; called on every frame while a key is held. |

One frame costs one `CopyBytesToJS` and one JS call. Never call into JavaScript per pixel:
it is hundreds of times slower and makes the preview useless for judging animation.

## Adapting a program

1. Move drawing and game/UI logic into files with no build tags and no `machine` import.
   They take a `Screen` (a `drivers.Displayer` plus `FillRectangle`) and buttons as
   `func() bool`. Touch or encoders become small interfaces of their own.
2. Keep one `setup()` per board in `hw_<board>.go` with the board's build tag (from the
   target JSON's `"build-tags"`), returning the same types.
3. Add `hw_wasm.go` with `//go:build tinygo && wasm`:
   ```go
   func setup() (Screen, [3]func() bool) {
       d := preview.NewDisplay(320, 240) // the real panel size
       return d, [3]func() bool{
           preview.Button("ArrowLeft", "z"),
           preview.Button("ArrowDown", "x"),
           preview.Button("ArrowRight", "c"),
       }
   }
   ```
4. Make sure the main loop sleeps every frame (`time.Sleep(33 * time.Millisecond)`), and
   calls `Display()` at the end of each frame.
5. Copy `preview/`, `web/`, `tools/` and the `Makefile` into the project; change the
   import path of `preview` to the project's module path; set `PKG` in the Makefile.

## Building and checking

```sh
tinygo build -o web/main.wasm -target=wasm -no-debug ./cmd/app
cp "$(tinygo env TINYGOROOT)/targets/wasm_exec.js" web/

# Headless: writes preview-out/frame-00300ms.png etc. and prints a JSON summary.
node tools/capture.cjs web/main.wasm --at 300,1500,2500 \
     --keys "500:ArrowLeft:80,1200:z:80" --scale 3
```

- `--at` takes snapshot times in milliseconds; `--duration` (default 2000) is how long the
  program runs. `--keys` is `start:key:hold` items; use `Space` for the space bar.
- The summary has `frames` (how many times `display()` ran), and per snapshot the number
  of distinct colours. `"blank": true` means every snapshot was one solid colour.
- Read the PNGs. Check the things the user will see first: text fits, nothing is cut off
  at the right or bottom edge, the state after each scripted key press is what the code
  intends.
- Every edit after a capture makes it stale. Capture again after the last change and
  report only those snapshots.
- For a human: `python3 -m http.server 8080 --directory web` (or `make serve`), then open
  http://localhost:8080. Fetching `main.wasm` as an `ArrayBuffer` avoids the MIME-type
  problem some static servers have with `instantiateStreaming`.
- `tinygo run -target=wasm` is not a substitute: it runs under Node without a page, so
  nothing provides `previewInit` and the program stops at the first call into it.

## What the preview cannot tell you

- **Speed.** The preview has a frame buffer; ST7789/ILI9341 do not. A design that calls
  `SetPixel` for the whole screen every frame looks fine here and crawls on hardware.
- **Panel settings.** Colour order, inversion, rotation, row/column offsets and gamma.
- **Memory.** The browser has gigabytes. Check `-size short` for the real target; frame
  buffers and images that fit here may not fit there.
- **Input feel.** Debounce, buttons that read low when pressed, touch calibration.
- **Sound and other peripherals.** Stub them in `hw_wasm.go` (for example a buzzer that
  does nothing, or one that logs with `println`).

## koebiten

koebiten 0.6.0 ships its own backend (`hardware/wasm.go`) and page (`static/`); its README
explains `make wasm` and `make serve`. `tools/capture.cjs` drives it without changes:
koebiten writes into `screenBuffer` (128×64 by default, `--size` to change) and reads
keys through `wasmKeyEvent`, which capture calls every 16 ms while a scripted key is
held. In koebiten's page, `z`/`Space`/`Enter` are button 0, `x` button 1, and the arrows
or `e`/`d`/`s`/`f` are the d-pad.
