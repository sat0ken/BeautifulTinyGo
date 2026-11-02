# TinyGo review rules

Each rule has an ID to cite in a review, what to look for, why it matters on a
microcontroller, how to confirm it, and the source a reader can look up. The writing-side
explanation of most rules is in the `tinygo-best-practices` skill (section numbers given as
§n). Rules come from TinyGo's documentation and source as of TinyGo 0.43, the
tinygo.org/x/drivers conventions (v0.36), Go Code Review Comments, and koebiten v0.6.0.

Source links used below:

- [Driver Design]: https://tinygo.org/docs/guides/driver-design/
- [Heap allocation]: https://tinygo.org/docs/concepts/compiler-internals/heap-allocation/
- [Optimizing binaries]: https://tinygo.org/docs/guides/optimizing-binaries/
- [Tips and Gotchas]: https://tinygo.org/docs/guides/tips-n-tricks/
- [Interrupts]: https://tinygo.org/docs/concepts/compiler-internals/interrupts/
- [volatile]: https://tinygo.org/docs/concepts/compiler-internals/volatile/
- [Porting]: https://tinygo.org/docs/guides/compatibility/
- [Language features]: https://tinygo.org/docs/reference/lang-support/
- [Datatypes]: https://tinygo.org/docs/concepts/compiler-internals/datatypes/
- [Differences from Go]: https://tinygo.org/docs/concepts/compiler-internals/differences-from-go/
- [CHANGELOG]: https://github.com/tinygo-org/tinygo/blob/release/CHANGELOG.md
- [drivers]: https://github.com/tinygo-org/drivers (CONTRIBUTING.md, AGENTS.md, sensor.go, displayer.go)
- [CRC]: https://go.dev/wiki/CodeReviewComments

---

## Device stability: crashes, hangs and wrong values

These are usually **Must fix**.

### TG-YIELD — Loops that wait or run forever must yield
- **Look for:** a goroutine or loop that never calls `time.Sleep`, `runtime.Gosched()`,
  or a blocking operation: empty delay loops (`for i := 0; i < N; i++ {}`), busy polling,
  `for {}` in a goroutine.
- **Why:** most microcontroller targets schedule goroutines cooperatively on one core.
  A goroutine that never yields keeps the CPU, and every other goroutine, including
  `main`, stops for good. The device looks frozen.
- **Confirm:** read the scheduler setting of the target (`"scheduler": "tasks"` in the
  target JSON) and trace when each goroutine first blocks.
- **Source:** [Tips and Gotchas] › Ensure Concurrency. §5.

### TG-BOUNDED-WAIT — Every wait has a fixed bound and reports failure
- **Look for:** `for !ready() {}`, `for bus.Tx(...) != nil {}`, waits for a pin or a status
  bit, start-up waits for a sensor or radio, with no retry limit or no error return. A
  loop that sleeps but retries forever is still unbounded.
- **Why:** a missing, unplugged or broken part turns the loop into a hang with no message.
- **Confirm:** ask what happens if the device never answers.
- **Source:** [Driver Design] › Hot loops (fixed bounds, yield, measured retry counts). §5.

### TG-ISR — Interrupt handlers only record the event
- **Look for:** in `Pin.SetInterrupt` callbacks and `interrupt.New` handlers: allocation
  (`append`, string building, `make`, maps, closures), printing (`println`, `fmt`),
  blocking (channel send without `select`/`default`, `time.Sleep`, bus transactions).
- **Why:** handlers run at interrupt level and can interrupt the allocator or the code
  they share data with. Allocation there can corrupt the heap; blocking can deadlock.
- **Fix shape:** set a flag or bump an atomic counter; do the work in the main loop.
- **Source:** [Interrupts] › Tips, Tricks and Gotchas. §5.

### TG-ISR-SHARED — State shared with an interrupt is atomic or volatile
- **Look for:** plain `bool`/`int` variables written by a handler and read by the main
  loop (or the reverse); slices, maps or strings modified from both sides.
- **Why:** the compiler may keep a plain variable in a register and never see the update,
  and a read-modify-write can lose events. A slice appended from both sides corrupts.
- **Fix shape:** `sync/atomic` types (`atomic.Bool`, `atomic.Uint32`) or
  `runtime/volatile`; a fixed ring buffer with atomic indices for data.
- **Source:** [volatile]; [Interrupts]. §5.

### TG-HEAP-GROWTH — Nothing grows without bound in a long-running loop
- **Look for:** `append` to a package-level slice, map inserts with new keys
  (especially time-stamped keys), caches, logs, histories, with no cap or eviction.
- **Why:** RAM is tens to hundreds of kilobytes with no swap. The program runs out of
  memory after minutes or hours, often after first fragmenting the heap.
- **Fix shape:** a fixed-size ring buffer, or drop the history.
- **Source:** [Driver Design] › Constrain memory use at compile time; [Heap allocation].
  §1, §3.

### TG-ADC-SCALE — `ADC.Get` returns 16-bit values on every chip
- **Look for:** dividing an `ADC.Get()` result by 1023, 1024, 4095 or 4096, or comparing it
  with thresholds taken from Arduino code.
- **Why:** TinyGo scales every chip's converter to 16 bits (0–65535; 0–65520 on ESP32
  family). Values computed against 10- or 12-bit ranges are 16 or 64 times too large.
- **Source:** TinyGo `src/machine`: `ADC.Get` doc comments ("returns the current value of an
  ADC pin in the range 0..0xffff", machine_nrf52xxx.go, machine_stm32_adc_f4.go;
  "scale raw reading to 16-bits", machine_rp2_adc.go; "returns 0..65520 (scaled from
  12-bit)", machine_esp32s3_adc.go). §6.

### TG-CONFIGURE — Pins and buses are configured before use, explicitly
- **Look for:** a pin read or written without `Configure`; buses configured without pins
  or frequency where the board has several options; `machine.InitADC()` missing before
  ADC use.
- **Why:** defaults differ per chip; an unconfigured pin may float or do nothing.
- **Source:** TinyGo `machine` package API; §6.

### TG-ERROR-PATH — Hardware errors are handled, and retries back off
- **Look for:** ignored `error` returns from `Configure`/`Tx`; `continue` on error with no
  sleep (a hot retry loop); init failures that let the program carry on with a
  half-configured device.
- **Why:** on a device nobody watches the log; a hot error loop also starves other work
  (TG-YIELD).
- **Source:** [CRC] › Handle Errors; [Driver Design] › Hot loops.

### TG-STACK — Goroutine stacks and large locals
- **Look for:** large local arrays (hundreds of bytes or more), deep recursion, `fmt` or
  big call chains inside goroutines.
- **Why:** each goroutine has a fixed stack (`default-stack-size` in the target JSON: 2 KB
  on generic Cortex-M, 8 KB on Pico and ESP32 targets). Overflow corrupts memory or
  crashes. Objects above the maximum stack object size go to the heap instead.
- **Source:** [Datatypes] › goroutine (`-stack-size`); [Driver Design] note on maximum
  stack object size.

### TG-RECOVER — Do not rely on recovering runtime panics
- **Look for:** `recover` used to survive nil dereferences, index errors or division by
  zero, or panics used for control flow.
- **Why:** before TinyGo 0.42 runtime panics could not be recovered, and some targets
  could not recover at all until 0.43 (Xtensa, WebAssembly without exceptions).
- **Source:** [CHANGELOG] 0.42 and 0.43; [Language features] › recover.

### TG-INT-WIDTH — Integer widths are explicit where overflow matters
- **Look for:** `int` used for register values, wire formats or timer arithmetic; code
  ported from AVR or Arduino that relies on 16-bit `int` or 32-bit `unsigned long` wrap.
- **Source:** [Porting] › SliceHeader section (AVR has 16-bit pointers, `int` is 32 bits on
  32-bit MCUs); §4.

---

## Memory and size

Allocation in a steady-state path is **Must fix** when the data grows (TG-HEAP-GROWTH) and
usually **Should fix** otherwise (fragmentation, GC pauses).

### TG-ALLOC-LOOP — No heap allocation per loop iteration, frame or scan
- **Look for:** allocations in the main loop, `Update`/`Draw`, key-scan and sensor-read
  paths, and anything they call.
- **Confirm:** both of these, because each misses things:
  - `tinygo build -target=<board> -print-allocs=<regexp>` lists allocations made by the
    compiler's escape analysis (`make`, `new`, composite literals, escaping locals,
    closures, values boxed into interfaces).
  - It does **not** list allocations made inside runtime helpers: string concatenation,
    `string([]byte)`, `strconv.Itoa`, `append` growth, map inserts. For those, use
    `testing.AllocsPerRun` in a host test run with `tinygo test` (TinyGo 0.43+), in a
    scratch copy of the code.
- **Source:** [Heap allocation]; [Driver Design] › Constrain memory use. §2, §3.

### TG-ALLOC-STRING — Building strings allocates
- **Look for:** `+` on strings, `string(b)`, `string(r)`, `strconv.Itoa`/`FormatFloat`,
  `fmt.Sprintf`, `time.Time.String()` in loops, frames or handlers.
- **Fix shape:** `strconv.AppendInt(buf[:0], v, 10)` into a fixed array kept in a struct;
  pass `unsafe.String(&buf[0], n)` to APIs that need a `string` and do not keep it.
- **Source:** [Heap allocation] (conversions, concatenation). §3.

### TG-ALLOC-BUS — Slices passed to bus interfaces escape
- **Look for:** in drivers, `bus.Tx([]byte{reg}, buf)` with a slice literal, or a local
  `make`/array, where `bus` is `drivers.I2C`, `drivers.SPI` or another interface.
- **Why:** the compiler cannot see which implementation will run, so it moves the slice's
  backing array to the heap on every call.
- **Confirm:** judge a driver by the general case, any implementation of the interface.
  `-print-allocs` may show nothing for a program that links only one implementation (for
  example only `*machine.I2C`), because TinyGo can then turn the interface call into a
  direct call. Linking a second implementation (a fake bus that keeps its argument) makes
  the allocations appear. Say this in the finding rather than dropping it.
- **Fix shape:** fixed read and write arrays in the `Device` struct; slice them per call.
- **Source:** [Driver Design] › Heap allocations: an introduction / Mitigations. §3, §7.

### TG-ALLOC-IFACE — Boxing values into interfaces allocates
- **Look for:** values larger than a pointer passed as `any`/interfaces in hot paths
  (`fmt` arguments, `println` of structs is fine, `errors` wrapping with values).
- **Source:** [Heap allocation] (interfaces); [Differences from Go].

### TG-FMT — Avoid `fmt` (and `log`) in firmware
- **Look for:** `import "fmt"` or `"log"` in code built for a microcontroller.
- **Why:** `fmt` is large and allocates; it often doubles a small program's flash use.
- **Fix shape:** `println` for debugging; `strconv.Append*` into buffers and
  `machine.Serial.Write` for formatted output; package-level `errors.New` sentinels.
- **Confirm:** build with and without and compare `-size short`.
- **Source:** [Optimizing binaries] › Do not import large packages. §4.

### TG-REFLECT — Avoid reflection-based packages
- **Look for:** `encoding/json`, `encoding/xml`, `text/template`, reflection-heavy
  third-party libraries.
- **Source:** [Porting] › Serialization and deserialization; [Language features] ›
  Reflection.

### TG-SLICEHEADER — Use `unsafe.Slice`/`unsafe.String`, not `reflect.*Header`
- **Source:** [Porting] › reflect.SliceHeader and reflect.StringHeader.

### TG-FLOAT — Prefer integers; `float32` over `float64`
- **Look for:** `float64` math in loops on chips without double-precision hardware (all
  common MCUs; RP2040, SAMD21, ESP32-C3/C6 have no FPU at all); float return values in
  driver APIs.
- **Fix shape:** fixed-point integers (milli-units); `float32` with a float32 math package
  if floats are needed.
- **Source:** [Driver Design] › General guidelines (floating point). §4.

### TG-CONST-DATA — Constant tables stay constant, and flat
- **Look for:** lookup tables, fonts, bitmaps built at start-up or written to after
  initialization; large tables whose elements hold slices (`[][]byte`, slices of structs
  with a slice field, tinyfont fonts).
- **Why:** TinyGo evaluates package-level initializers at compile time and can keep data
  that is never written in flash instead of RAM, but a table of slices is copied into RAM
  even when nothing writes to it (TinyGo 0.43: 8 KB as `[][]byte` is 8 KB of `.data`; the
  same bytes as `[]byte`, `[]string` or a `//go:embed` string are none).
- **Fix shape:** one flat `[]byte` with an offset table, `[]string`, or `//go:embed` into a
  `string`; fonts stored as strings.
- **Confirm:** the `data` column of `-size short`, before and after.
- **Source:** [Differences from Go] (globals); `tinygo-best-practices`
  `references/tinygo-memory-and-size.md` (measurements). §3.

### TG-SIZE — Size is measured, and big wins are taken first
- **Confirm:** `tinygo build -target=<board> -size short` (or `-size full` per package).
- **Source:** [Optimizing binaries]. §10.

---

## Drivers (tinygo.org/x/drivers style)

Strict for code meant to be shared or upstreamed; **Should fix** unless it causes a bug.

### TG-DRV-MACHINE — Driver packages do not import `machine`
- **Look for:** `import "machine"`, `machine.Pin` or `*machine.I2C` in a driver's API.
- **Why:** the drivers are also used with the standard Go toolchain, where `machine` does
  not exist; it also ties the driver to one board.
- **Fix shape:** take `drivers.I2C`/`drivers.SPI`; pins as `func(level bool)` (output) and
  `func() bool` (input).
- **Source:** [Driver Design] › Driver for a peripheral. (Some older drivers still take
  `machine.Pin`; new code should not.)

### TG-DRV-SHAPE — One package per device, `Device`, `New`, `Configure(Config) error`
- **Look for:** missing `Config`/`DefaultConfig`; raw integers for register-backed options
  instead of named types with datasheet values; codependent options without validation;
  constructors that talk to the hardware.
- **Source:** [Driver Design] › Driver for a peripheral.

### TG-DRV-OPTIONS — No functional options or option interfaces
- **Source:** [Driver Design] › Driver for a peripheral ("higher overhead in performance,
  memory and binary size").

### TG-DRV-CONCURRENCY — No goroutines, channels or `select` in drivers
- **Source:** [Driver Design] › General guidelines.

### TG-DRV-SENSOR — Sensors follow `drivers.Sensor`
- **Look for:** sensors whose read methods both talk to the bus and return values, with no
  `Update(which drivers.Measurement) error` and value getters.
- **Source:** [Driver Design] › General guidelines and the HSC example; [drivers] sensor.go.

### TG-DRV-BATCH — Few, large bus transactions
- **Look for:** byte-at-a-time `Tx` calls, toggling a data/command pin that is already set.
- **Source:** [Driver Design] › Consolidate I/O into as few dynamic calls as possible.

### TG-DRV-UPSTREAM — Ready for tinygo-org/drivers
- **Look for:** missing example under `examples/`, missing README device entry, comments
  that break the repository's AGENTS.md rules, branch not based on `dev`.
- **Source:** [drivers] CONTRIBUTING.md, AGENTS.md.

---

## Displays

### TG-DISP-SETPIXEL — No per-pixel drawing on unbuffered panels
- **Look for:** loops calling `SetPixel` over large areas on `st7789`, `ili9341`, `st7735`
  and similar SPI panels.
- **Why:** on these drivers `SetPixel` is a 1×1 `FillRectangle`: a window command and a
  separate SPI transaction per pixel. A full 240×240 clear is 57,600 transactions.
- **Fix shape:** `FillRectangle`, `FillScreen`, `DrawRGBBitmap8`, `DrawBitmap`.
- **Source:** [drivers] st7789.go (`SetPixel` calls `FillRectangle(x, y, 1, 1, c)`). §8.

### TG-DISP-REDRAW — Redraw only what changed
- **Look for:** clearing and repainting the whole screen every frame on SPI panels.
- **Why:** slow and flickers; dirty flags or rectangles cost little.
- **Source:** §8.

### TG-DISP-DISPLAY — `Display()` once per frame
- **Look for:** buffered displays (ssd1306, framebuffers, the WebAssembly preview) drawn
  without `Display()`.
- **Source:** [drivers] displayer.go (`Display sends the buffer (if any) to the screen`). §8.

### TG-DISP-RAM — Frame buffers fit the chip
- **Look for:** full-screen RGB565 buffers (240×240 = 115,200 bytes, 320×240 = 153,600) on
  chips with less free RAM.
- **Confirm:** `-size short` RAM column plus the buffer.
- **Source:** §8.

### TG-PREVIEW — Display code can be checked without the board
- **Look for:** drawing code that imports `machine` directly, no `-target=wasm` path or
  equivalent, so layout bugs are only found on hardware. If the project already has its
  own preview or golden-image tests, check that new screens are covered by them.
- **Source:** BeautifulTinyGo `tinygo-best-practices` §9.

---

## Structure, tests and hardware facts

### TG-MACHINE-EDGE — `machine` stays at the edge
- **Look for:** logic, drawing or protocol code importing `machine`, so it cannot be tested
  on the host or reused on another board.
- **Source:** §1, §11.

### TG-HW-SOURCE — Hardware facts name their source
- **Look for:** pin numbers, register addresses and values, timings and calibration
  constants with no reference to a board file, datasheet section, schematic or vendor code.
- **Why:** they cannot be checked, and a wrong guess only shows up on hardware. Never
  recommend changing such a value without a primary source either.
- **Source:** §1 ("Never guess hardware facts").

### TG-TEST-HOST — Logic is tested on the host
- **Look for:** pure logic (parsers, conversions, state machines, CRCs, game rules) with no
  `go test`/`tinygo test` tests; loop bodies with no `AllocsPerRun` test.
- **Source:** §12.

### Go idioms
Cite the section of Go Code Review Comments directly, for example "[CRC] › Error Strings",
"› Doc Comments", "› Receiver Names", "› Interfaces", "› Handle Errors", "› Package
Names". These apply unchanged, except where a rule above takes precedence (for example
`fmt.Errorf` wrapping in device code, see TG-FMT).

---

## koebiten games

### KB-TICK — koebiten ticks every 32 ms
- **Look for:** speeds, timers and durations that assume 60 updates per second.
- **Source:** koebiten v0.6.0 `koebiten.go` (`RunGame`: `time.Tick(32 * time.Millisecond)`).

### KB-WIDTH — gopher-board's logical screen is 100×64
- **Look for:** layouts that assume 128 pixels on the `gopher_board_spi` target.
- **Source:** koebiten v0.6.0 `hardware/gopher-board-spi.go` (`Size()` returns 100, 64).

### KB-SPLIT — Logic in `Update`, drawing in `Draw`
- **Source:** koebiten `game.go` (`Game` interface documentation).

### KB-HUD — HUD text must not allocate per frame
- **Look for:** `"Score: "+strconv.Itoa(n)` and similar in `Draw` (koebiten's own sample
  games do this).
- **Source:** TG-ALLOC-STRING; `koebiten-games` skill §6.

### KB-PIXEL-HELPERS — No koebiten `Draw*` helper per pixel
- **Why:** each helper call checks its destination with reflection.
- **Source:** koebiten v0.6.0 `koebiten.go` (`isNil` uses `reflect.ValueOf`).

---

## Ports from Arduino, Arduboy, ESP-IDF, Moddable and C game SDKs

### PORT-BEHAVIOR — Observable behavior matches the original
- **Look for:** serial output format and rounding, timing and schedules, debounce, analog
  scaling (TG-ADC-SCALE), byte order of 16-bit bus values, sensor units, persistence, and
  error behavior that differ from the original without a note in PORTING.md.
- **Source:** `tinygo-port` skill §1.

### PORT-FPS — 60 FPS games are reconciled with koebiten's tick
- **Look for:** Arduboy frame counts and speeds used unchanged with one logic frame per
  32 ms tick (the game runs at half speed).
- **Source:** `tinygo-port` skill §7; KB-TICK.

### PORT-TASKS — Tasks and callbacks are mapped onto the cooperative scheduler
- **Look for:** one goroutine per FreeRTOS task or JavaScript timer that polls without
  blocking; code that relies on task priorities, preemption or a second core;
  `xQueueSendFromISR` turned into a blocking channel send inside an interrupt handler;
  `onReadable`/`onChanged` bodies run inside the interrupt; callbacks whose order changed;
  a slow step (a sensor conversion wait, a network call) that now delays a step the
  original ran in parallel, with no note of the worst case.
- **Why:** TinyGo on ESP chips (and most targets) runs goroutines cooperatively on one
  core: a non-blocking loop starves the rest, and handlers run at interrupt level, where
  Moddable and FreeRTOS deferred the work to a task or the event loop.
- **Confirm:** list the original's tasks, ISRs and callbacks with their periods, and find
  each one in the port (a loop step, a goroutine with a blocking call, a handler that
  only records). Trace when each goroutine first blocks (TG-YIELD).
- **Source:** `tinygo-port` skill §4–§6, `references/espidf-to-tinygo.md` (FreeRTOS →
  goroutines and the main loop), `references/moddable-to-tinygo.md` (What changes
  underneath); TG-ISR; [Interrupts].

### PORT-PLATFORM — Services the original platform provided are replaced or reported
- **Look for:** behavior an ESP-IDF or Moddable program got from its platform and the port
  silently lost: `ESP_ERROR_CHECK` reboots and watchdog resets (TinyGo disables the ESP
  watchdogs, so a hang stays hung); internal I2C pull-ups the IDF driver enabled; LEDC duty
  values written for the original `duty_resolution` instead of scaled by `pwm.Top()`; ADC
  attenuation and `adc_cali` calibration; NVS or `Preference` data and wear levelling;
  buffers sized for PSRAM; SMBus 16-bit reads that were little-endian by default; sensor
  class units (humidity as a fraction); pins taken from a Moddable host provider with no
  source named.
- **Why:** these differences change values or failure behavior on the device without any
  compile error.
- **Confirm:** compare against the original's `sdkconfig`, driver configuration structs,
  `manifest.json` and host provider; check PORTING.md lists each difference.
- **Source:** `tinygo-port` skill §5–§6 and its two references; TinyGo
  `src/machine/machine_esp32*`, `src/runtime/runtime_esp32*.go`.

### PORT-C-SEMANTICS — C arithmetic and evaluation order are kept
- **Look for:** in a port from C or C++: `short` or `unsigned char` values that wrap,
  turned into `int`; time differences on `uint32` microseconds compared without the
  original's signed cast (`(int)(Time() - t) > n`); `float` math (`sinf`, `atan2f`,
  `powf`) kept in `float64`; a call whose arguments both draw random numbers (or have other
  side effects), ported in another order than the original compiler evaluated them; slots
  of an object array reset as a whole where the C set only some fields; a quirk of the
  original changed without a note.
- **Why:** each changes values without a compile error, and in a game the difference grows
  frame after frame.
- **Confirm:** compare with the original compiled on the host, with a stub header, the
  same random generator and a scripted input, by checksums of the state per frame.
- **Source:** `tinygo-port` skill §3, §4 and §8; `references/c-games-to-tinygo.md`.

### PORT-ASSETS — Third-party pictures, data and titles are cleared
- **Look for:** a port of permissively licensed code that ships pictures, logos, level
  or music data, or a title that belong to someone else (characters from films, a
  commercial game's logo or sprites, levels from another game); parts of the original
  under another license (a GPL directory in an Unlicense SDK).
- **Why:** the license of the code does not cover assets its author does not own.
- **Fix shape:** leave the asset or the game out, or draw a replacement at the same size
  and frame count and rename the title; list it in PORTING.md.
- **Source:** `tinygo-port` skill §9; `references/c-games-to-tinygo.md` (Assets and
  rights).

### PORT-LICENSE — The original's license and notice are kept
- **Look for:** code translated from a licensed original (Arduino library, Arduboy game,
  ESP-IDF or a component, Moddable module or example) with no notice; LGPL code (the
  Moddable runtime and its sensor drivers) translated into a package released under a
  different license; CC BY examples ported without attribution; code with no license
  redistributed.
- **Source:** `tinygo-port` skill §9; the original project's license.
