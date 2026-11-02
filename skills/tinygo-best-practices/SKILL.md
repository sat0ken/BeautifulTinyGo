---
name: tinygo-best-practices
description: >-
  Write small, robust, idiomatic TinyGo for microcontrollers and TinyGo WebAssembly. Use
  this skill whenever you write, review, refactor, debug, or explain code that is built
  with TinyGo: firmware for boards such as Raspberry Pi Pico/RP2040/RP2350, ESP32/ESP32-S3,
  M5Stack, Seeed XIAO, Wio Terminal, nRF52, SAMD or STM32; code that imports "machine",
  "device/...", "runtime/interrupt" or tinygo.org/x/drivers, tinyfont, tinydraw or
  tinyterm; drivers for I2C/SPI/UART sensors and displays; keyboards and other USB HID
  devices; anything with `tinygo build`, `tinygo flash`, `tinygo monitor`, a target JSON,
  or `-target=wasm`. Trigger even when the user only says "Go" but mentions a board, a
  pin, a display, a sensor or flashing. For programs that draw on a display it requires a
  WebAssembly preview that is checked before the code is handed back. Pairs with the
  koebiten-games and tinygo-port skills, and with tinygo-review for review-only requests.
---

# TinyGo best practices

Distilled from the TinyGo documentation (Driver Design and Development, Heap allocation,
Optimizing binaries, Tips/Tricks/Gotchas, Go language features, Porting code to TinyGo,
Interrupts, The volatile keyword), the conventions of tinygo.org/x/drivers, and Effective
Go / Go Code Review Comments. A digest of each is in `references/`. Where general Go advice
and microcontroller reality disagree, this file says which wins and why.

Written against **TinyGo 0.43** (October 2026) and tinygo.org/x/drivers 0.36. Run
`tinygo version` first. If the project uses an older release, say so when advice depends
on the version (panic recovery is the common case, see §4).

## 1. Mindset

- **The program is small and runs for a long time with no operating system under it.**
  RAM is tens to hundreds of kilobytes, there is no swap, and a crash means a dead device
  on someone's desk. Code for that, not for a server.
- **Allocate at start-up, then stop.** Heap allocation during initialization is fine. The
  steady-state loop (every frame, every key scan, every sensor read) should allocate
  nothing. Fragmentation from a slow trickle of allocations is the classic way a TinyGo
  program dies after an hour.
- **Big-Go habits do not carry over.** `fmt` everywhere, `encoding/json`, reflection-heavy
  libraries, a goroutine per concern, channels as the default glue, functional options,
  `float64` math, unbounded "wait until ready" loops: each costs flash, RAM or
  reliability on a microcontroller. Use them only when you can say why they are needed.
- **The target build is the test that counts.** Code that compiles with `go build` on a
  laptop can still fail to build, not fit, or misbehave under TinyGo. Build for the
  user's real target every time.
- **Never guess hardware facts.** Pin numbers, SPI/I2C buses, display controllers, panel
  offsets and button polarity come from the board file in
  `$(tinygo env TINYGOROOT)/src/machine/board_<name>.go`, the target JSON, the
  schematic, or the user. Read them; quote where they came from.
- **Keep `machine` at the edge.** Put all hardware set-up in thin `hw_<board>.go` files.
  The rest of the program sees small interfaces and plain functions. That one decision
  makes the logic testable with `go test`, previewable in a browser, and portable to the
  next board.

## 2. Before handing code back

Run these and fix what they report. If something cannot be run (no hardware, a backend
missing from the local LLVM), say exactly what was not verified.

1. `gofmt -l .` prints nothing. Run `go vet ./...` on packages that also build with the
   normal Go toolchain (pure logic packages).
2. **Build for the real target and read the size:**
   `tinygo build -o /tmp/out.uf2 -target=<board> -size short ./cmd/app`. If the user has
   not named the board and it matters, ask; otherwise state the target you assumed.
   When you refactor or optimize existing code, build the original first with the same
   command, and report flash and RAM before and after.
3. **Check allocations, two ways.**
   - `tinygo build -target=<board> -print-allocs=<pkg-regexp> ./cmd/app` (for example
     `-print-allocs=main` or `-print-allocs='mydriver|main'`). Nothing should appear inside
     the main loop, a per-frame or per-scan function, or an interrupt handler.
   - `-print-allocs` only shows allocations the compiler's escape pass handles (`make`,
     `new`, composite literals, escaping locals, closures). It does **not** show
     allocations made inside runtime helpers: string concatenation, `[]byte`↔`string`
     conversion, `append` growth, map inserts. For a loop body that must not allocate,
     add a host test with `testing.AllocsPerRun` and run it with `tinygo test`
     (TinyGo 0.43 or later): `if n := testing.AllocsPerRun(30, frame); n != 0 { t.Fatal(n) }`.
   - Explain any allocation you leave in place.
4. **Programs that draw on a display:** build the WebAssembly preview, run the headless
   capture, and look at the PNGs before you report success (§9). A blank frame or a
   layout that falls off the screen is a failure, not a detail. Captures go stale with
   every edit: the snapshots you report must come from the final build.
5. **Host tests** for logic packages: `go test ./...` (or `tinygo test ./pkg` when the
   package uses TinyGo-only APIs that still run on the host).
6. **Do not flash** unless the user asks. When asked: `tinygo flash -target=<board>
   -monitor ./cmd/app`.
7. Report: what was built, for which target, flash/RAM from `-size short`, allocation
   status, preview snapshots (for display programs), and what still needs a real board.
   When you fix a hang, crash or slowdown, name the mechanism in TinyGo terms (for
   example: a goroutine that never yields starves the cooperative scheduler; a heap that
   grows every loop fragments and runs out; an interrupt handler that blocks), not only
   the symptom you removed.

## 3. Memory and allocation

TinyGo moves allocations to the stack when escape analysis can prove they do not escape.
Know what defeats it (details in `references/tinygo-memory-and-size.md`):

- **Pointers that escape.** Taking `&local` and storing it in a global, a struct that
  outlives the call, or an interface forces a heap allocation.
- **Slices passed through interfaces.** `bus.Tx([]byte{reg, 0}, buf[:])` on a
  `drivers.SPI` or `drivers.I2C` interface escapes both slices, because the compiler
  cannot see the callee. Keep fixed buffers in the device struct and slice them:
  `d.w[0] = reg; d.bus.Tx(d.w[:1], d.r[:2])`.
- **Conversions:** `string`↔`[]byte` (except a read-only string→bytes conversion the
  compiler can prove), `string(rune)`, string concatenation, `strconv.Itoa`,
  `fmt.Sprintf`. Format into a fixed array with `strconv.AppendInt(d.txt[:0], v, 10)`.
  Slicing an existing string (`digits[i : i+1]`) does not allocate. To hand formatted
  text to an API that wants a `string` (tinyfont, koebiten), pass
  `unsafe.String(&d.txt[0], n)`: a view of the array, no copy. Use it at once and never
  keep it, because the next format overwrites the bytes.
- **Interfaces holding values bigger than a pointer**, closures that capture more than a
  pointer's worth of variables, creating or growing maps and slices, starting
  goroutines.
- **Large locals.** Objects above TinyGo's stack-object size limit (a few hundred bytes)
  go to the heap. A 1 KB scratch array belongs in a struct that is allocated once.
- **Constant tables go in flash, if they are flat.** TinyGo evaluates package-level
  initializers at compile time when it can, and data that is never written can stay in
  flash instead of RAM. Keep fonts, bitmaps, note tables and lookup tables as
  package-level arrays and do not write to them. But a table whose elements hold slices
  (`[][]byte`, a slice of structs with a `[]byte` field, the glyph tables of tinyfont's
  fonts) is copied into RAM even when nothing writes to it. Keep big tables flat (one
  `[]byte` and a table of offsets) or make them strings (`[]string`, or one `string` with
  offsets), and load large binary data with `//go:embed` into a `string`. The `data`
  column of `-size short` shows what went to RAM (measurements in
  `references/tinygo-memory-and-size.md`).
- The garbage collector is much slower than big Go's and works less well on very small
  chips (AVR) and WebAssembly. Design so it rarely has anything to do.

## 4. Standard library and language support

- **Check before importing.** The TinyGo site lists the standard packages that compile
  (`reference/lang-support/stdlib`). "Compiles" does not mean "works fully": reflection
  is partial and maps are slower than in big Go.
- **Avoid on microcontrollers:** `fmt` (large; use `println` and `strconv.Append*`),
  `encoding/json` and other reflection-based encoders (write a small explicit encoder or
  use a code-generated one), `regexp`, `text/template`, `log` (pulls in `fmt`). Prefer
  `errors.New` sentinels declared at package level over `fmt.Errorf` in device code.
- **Panic recovery is version-dependent.** TinyGo 0.42 made runtime panics (nil
  dereference, divide by zero, index out of range, map and channel panics) recoverable,
  and 0.43 added recovery on Xtensa (ESP32) and on WebAssembly without the exceptions
  proposal. Older releases cannot recover runtime panics and some targets could not
  recover at all. Either way, do not use panic/recover for control flow; return errors.
- **Use `unsafe.Slice`, `unsafe.String`, `unsafe.SliceData`, `unsafe.StringData`**, never
  `reflect.SliceHeader`/`StringHeader` (their field types differ in TinyGo).
- **Integer sizes:** `int` is 32 bits on 32-bit microcontrollers, and pointers are 16 bits
  on AVR. Use sized types (`uint8`, `int16`, `uint32`) for register values, wire formats
  and anything that must not overflow differently between targets.
- **Floating point:** Cortex-M0+ (RP2040, SAMD21) and the RISC-V ESP32-C3/C6 have no
  FPU, so all float math there is software. ESP32, ESP32-S3, SAMD51 and RP2350 have
  single-precision hardware; `float64` is software almost everywhere. Prefer integers or
  fixed point; when you need floats, use `float32` (and a `float32` math package such as
  `github.com/chewxy/math32`) rather than `float64`.

## 5. Concurrency, scheduling and interrupts

- **The scheduler is cooperative on most microcontroller targets** (`-scheduler=tasks`),
  usually on one core. A goroutine that never blocks starves every other goroutine. Any
  loop that waits must call `time.Sleep` or `runtime.Gosched()`.
  RP2040/RP2350 also have `-scheduler=cores` for two cores; use it deliberately and treat
  shared state as truly concurrent.
- **Every goroutine costs a heap-allocated stack** (`default-stack-size` in the target
  JSON: 2 KB for generic Cortex-M, 8 KB for Pico and ESP32 targets; `-stack-size`
  overrides it). Prefer one main loop driving explicit state machines. Use a goroutine for
  genuinely independent blocking work, such as a USB or network stack.
- **Bound every wait.** "Poll until ready" loops get a retry limit and a yield, and return
  an error when they give up:
  `for retries := 100; retries > 0 && !d.ready(); retries-- { time.Sleep(time.Millisecond) }`.
  This includes start-up waits for a sensor or radio. A loop that sleeps but retries
  forever is still unbounded: give up after a fixed number of tries, return an error, and
  let the caller decide whether to report it, retry later, or stop.
- **Interrupt handlers** (`pin.SetInterrupt` callbacks, `interrupt.New` handlers) run at
  interrupt level. Inside them: no allocation, no blocking (no channel send without
  `select`/`default`, no `time.Sleep`), no `println`/`fmt`. Record the event (set a flag,
  bump a counter, push into a fixed ring buffer) and do the work in the main loop.
- **State shared with an interrupt** must use `sync/atomic` or `runtime/volatile`;
  ordinary variables may be cached by the compiler.
- `interrupt.New` does not accept closures over local variables. Bind a method with a
  pointer receiver on a global value.
- **WebAssembly uses the asyncify scheduler.** The main loop must sleep every frame or
  the browser tab freezes.

## 6. The machine package and boards

- **Configure before use.** Pins: `p.Configure(machine.PinConfig{Mode: machine.PinOutput})`
  (`PinInput`, `PinInputPullup`, `PinInputPulldown`). Buses: `machine.I2C0.Configure(
  machine.I2CConfig{SCL: ..., SDA: ..., Frequency: 400_000})`,
  `machine.SPI0.Configure(machine.SPIConfig{SCK: ..., SDO: ..., SDI: ..., Frequency: ...})`.
  Give pins and frequencies explicitly; defaults differ by chip.
- **Use the board's names**: `machine.LED`, `machine.BUTTON_A`, `machine.SPI0_SCK_PIN`,
  `machine.LCD_DC_PIN`. They exist only on boards that define them; read the board file.
- **ADC values are scaled to 16 bits.** After `machine.InitADC()` and
  `a.Configure(machine.ADCConfig{})`, `a.Get()` returns 0–65535 on every chip, whatever
  the converter's real resolution. Scale from 65535, not 1023/4095.
- **PWM** is per peripheral group: `pwm.Configure(machine.PWMConfig{Period: ns})`,
  `ch, _ := pwm.Channel(pin)`, `pwm.Set(ch, pwm.Top()*duty/100)`. Which group drives
  which pin is chip-specific; check the chip's machine file.
- **I2C:** `Tx(addr, w, r)`, `ReadRegister(addr, reg, buf)`, `WriteRegister(addr, reg, buf)`.
  **SPI:** `Tx(w, r)`, `Transfer(b)`. Batch bytes into one `Tx` call.
- **Serial:** `machine.Serial` is USB CDC on boards with native USB and a UART otherwise;
  `println` writes to it. Watch it with `tinygo monitor` or `tinygo flash -monitor`.
- **Board-specific files use the build tags from the target JSON** (`"build-tags"`), for
  example `//go:build m5stack` or `//go:build waveshare_rp2040_zero`. For a custom board,
  write a target JSON that `"inherits"` the closest existing target and adds a build tag
  and `"serial-port"`, then pass `-target ./targets/myboard.json`.

## 7. Writing drivers

Follow TinyGo's Driver Design guide (digest: `references/tinygo-driver-design.md`):

- **One package per device** (`mpu6050`), or per family with shared logic (`as560x`).
- **A `Device` type** holds the bus, pins and buffers; `New(bus, ...)` constructs it and
  `Configure(cfg Config) error` sets it up. Provide `DefaultConfig()`; give enum-like
  config fields their own types with datasheet values; validate codependent fields.
- **Do not import `machine` in a driver.** Take `drivers.I2C`/`drivers.SPI` and pins as
  `func(level bool)` / `func() bool`, so big Go and other boards can use the package.
  (Some older drivers in tinygo.org/x/drivers still take `machine.Pin`; do not copy that.)
- **No functional options** and no option interfaces: they cost size and speed, and a chip's
  register map never changes after manufacture.
- **Integers in the public API** (milli-degrees as `int32`, not `float64` degrees). Use
  floats internally only if you must; the API can then drop them later without breaking
  callers.
- **No goroutines, channels or `select` inside drivers.** Add a mutex only to methods that
  are realistically shared between goroutines.
- **Zero allocations per call:** buffers live in `Device`, slices are cut from them,
  repeated single-byte writes are merged into one bus transaction.
- **Sensors** implement the `drivers.Sensor` shape: `Update(which drivers.Measurement)
  error` reads the hardware, getter methods return the last values.
- Contributing upstream: branch from `dev`, add an example under `examples/`, add the
  device to the README list, and follow the repository's `AGENTS.md` (plain, short
  comments; no tool attributions).

## 8. Displays

- **Know whether the driver is buffered.** `ssd1306` keeps a frame buffer: `SetPixel`
  is cheap and `Display()` sends the frame. `st7789` and `ili9341` are unbuffered:
  every `SetPixel` is a separate SPI transaction and `Display()` does nothing. Full-screen
  `SetPixel` loops on those panels take whole seconds.
- **On unbuffered panels draw in blocks**: `FillRectangle`, `DrawRGBBitmap8`,
  `FillRectangleWithBuffer`, `DrawBitmap(pixel.Image)`. Track what changed and redraw
  only that (dirty flags or rectangles). Clear the screen once, not every frame.
- **Budget RAM before choosing a frame buffer.** 240×240 RGB565 is 115,200 bytes,
  320×240 is 153,600: fine on RP2040 or ESP32-S3 (with care), impossible on a 32 KB SAMD21.
- **When an RGB565 frame buffer does not fit, keep palette indices.** One byte a pixel
  (76,800 bytes at 320×240) and a palette of 256 RGB565 colours: give a colour an index
  the first time it is drawn, use the nearest entry once all 256 are taken, and free them
  all when the whole screen is cleared. In `Display()`, convert only the changed
  rectangle, a band of a few rows at a time, into a small RGB565 buffer for
  `DrawRGBBitmap8` (two buffers if the driver sends one by DMA while you fill the other).
  For pictures with more than 256 colours, choose the palette ahead of time for each
  screen (median cut). On an ESP32 without PSRAM this left about 104 KB of heap where a
  320×240 RGB565 buffer left 38 KB (github.com/sat0ken/tinygo-esp32-cyd, `picopad`).
- **Call `Display()` at the end of every frame** even on unbuffered panels: it is free
  there, and buffered panels and the browser preview need it.
- Let the application depend on a small interface such as
  `interface{ drivers.Displayer; FillRectangle(x, y, w, h int16, c color.RGBA) error }`,
  which st7789, ili9341, ssd1306 and the preview display all satisfy.
- Text: `tinyfont.WriteLine(display, &font, x, y, str, color)`; shapes: `tinydraw`;
  a scrolling console: `tinyterm`.
- Colour order (RGB/BGR), inversion, rotation and row/column offsets are panel settings in
  the driver `Config`. They only show up on real hardware; list them as unverified.

## 9. Display programs: preview in WebAssembly before flashing

**Rule: every program that draws on a display also builds for `-target=wasm`, and you
look at its output before handing it back.** The template in `assets/wasm-preview/` has
everything; `references/wasm-preview.md` explains it.

1. **Split the program:** logic and drawing in files without build tags that never import
   `machine`; `hw_<board>.go` per board with the board's build tag; `hw_wasm.go` with
   `//go:build tinygo && wasm` that returns `preview.NewDisplay(w, h)` and
   `preview.Button("ArrowLeft", "z")` style buttons. Use the real panel size.
2. **Copy the template** (`preview/`, `web/`, `tools/capture.cjs`, `Makefile`) into the
   project and fix the `preview` import path to the user's module.
3. **Build:** `tinygo build -o web/main.wasm -target=wasm -no-debug ./cmd/app` and copy
   `$(tinygo env TINYGOROOT)/targets/wasm_exec.js` into `web/`. It must come from the same
   TinyGo that built the wasm; Go's own `wasm_exec.js` does not work.
4. **Check it yourself, no browser needed:**
   `node tools/capture.cjs web/main.wasm --at 300,1500 --keys "500:ArrowLeft:80"`.
   Read every PNG it writes. The JSON summary reports `"blank": true` when nothing was
   drawn. Script the key presses that reach each screen you changed. After your last
   edit, rebuild and capture again; report only snapshots of the final build, and say
   which snapshot shows which screen.
5. **Give the user a way to play it:** `make serve`, then open http://localhost:8080
   (keyboard keys stand in for buttons).
6. **Still build the hardware target** (§2) and say what the preview cannot show: drawing
   speed on SPI panels, colour order and offsets, touch, sound, RAM limits.

koebiten games already have a WebAssembly backend; use the koebiten-games skill for them.

## 10. Size and speed

- Default optimization is for size (`-opt=z`). `-size short` (or `-size full`) shows what
  takes space; the usual culprits are `fmt`, reflection, `math/big`, big fonts and images.
- Size levers, in order of safety: drop large imports; `-panic=trap`;
  `-scheduler=none` (only with no goroutines); `-gc=leaking` (only for short-lived
  programs). `-no-debug` shrinks WebAssembly a lot and does nothing for firmware.
- Speed levers: `-opt=2`, fewer interface calls in hot loops, block transfers instead of
  per-pixel or per-byte calls, integer math.
- Measure first: time a hot loop with `time.Now()` and print outside the loop.

## 11. Project layout and builds

- Logic packages (`game/`, `keymap/`, `protocol/`) with no `machine` import; one `cmd/`
  or `main` package per program; `hw_<board>.go` files selected by build tags;
  `targets/*.json` for custom boards.
- Pin dependency versions in `go.mod` (`tinygo.org/x/drivers`, `tinyfont`, `koebiten`).
  Note the TinyGo version in the README; check
  `reference/go-compat-matrix` when changing Go versions (TinyGo 0.43 accepts Go 1.25–1.27).
- A Makefile or script that builds every supported board, like koebiten's `smoketest`, is
  the cheapest CI there is.

## 12. Testing and debugging

- Design for host tests: pure logic gets table-driven `go test` tests, hardware is passed
  in as interfaces or functions that tests can fake.
- `println` and `tinygo monitor` are the first debugger; print outside hot loops and
  interrupt handlers. An LED blink is the debugger inside an interrupt handler.
- `-print-allocs`, `-size full`, `-opt=1` (with GDB/OpenOCD or a probe) for deeper work.
- Flashing problems (port not found, bootloader mode, permissions): see TinyGo's
  "Tinygo flash errors" guide before changing code.

## 13. Go idioms that still apply

Effective Go and Go Code Review Comments still describe good TinyGo code
(`references/go-idioms.md`): short, clear names; doc comments on exported identifiers;
small interfaces defined by the consumer; errors returned, not panicked; early returns;
no stuttering names (`st7789.Device`, not `st7789.ST7789Device`). Where they assume a
server (fmt-based error wrapping everywhere, goroutines as free, maps as the default
container), the rules above take precedence.

## 14. Final checklist

- [ ] Builds for the user's real target; flash/RAM reported from `-size short` (before and
      after, for refactors).
- [ ] No allocations in the main loop, per-frame/per-scan paths or interrupt handlers
      (`-print-allocs` checked, and `testing.AllocsPerRun` for loop bodies).
- [ ] No `fmt`, `encoding/json` or reflection-heavy imports without a stated reason.
- [ ] Large constant tables are flat or strings, so they stay in flash (`data` column of
      `-size short` checked).
- [ ] Every wait is bounded and yields; no busy loop without `time.Sleep`/`Gosched`.
- [ ] Interrupt handlers only record events; shared state is atomic or volatile.
- [ ] `machine` appears only in `hw_*.go`/`main` set-up; logic is host-testable.
- [ ] Pins and buses come from the board file or the user, with the source named.
- [ ] Drivers: `Device` + `Config` + `Configure`, no `machine` import, integer API,
      buffers in the struct.
- [ ] Display programs: WebAssembly preview built, snapshots of the final build captured
      and looked at, `make serve` instructions given, hardware-only risks listed.
- [ ] Fixes explained by their TinyGo mechanism, not only by the symptom.
- [ ] Anything not verified on hardware is listed plainly.
