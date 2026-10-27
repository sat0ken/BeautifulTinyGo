# Digest: language support, porting Go code, interrupts and versions

Sources (paraphrased for this skill):

- Go language features: https://tinygo.org/docs/reference/lang-support/
- Packages supported by TinyGo: https://tinygo.org/docs/reference/lang-support/stdlib/
- Porting code to TinyGo: https://tinygo.org/docs/guides/compatibility/
- Interrupts: https://tinygo.org/docs/concepts/compiler-internals/interrupts/
- The volatile keyword: https://tinygo.org/docs/concepts/compiler-internals/volatile/
- Go compatibility matrix: https://tinygo.org/docs/reference/go-compat-matrix/
- Release notes: TinyGo 0.42 (September 2026) and 0.43 (October 2026) CHANGELOG

## Language support

TinyGo implements all major language features, but details differ and the project itself
says the only reliable answer to "does X work" is to try it.

- **cgo:** Clang is embedded to read `import "C"`, but some cgo features, including parts of
  `#cgo` lines, are partial.
- **Reflection:** reimplemented and mostly working, but incomplete. Packages built on it
  (notably `encoding/json`) are the most common porting failure.
- **Maps:** work, but can be slower than expected; some key types are hashed or compared
  through reflection.
- **Standard library:** many packages compile; "compiles" does not imply "fully works".
  Check the supported-packages page.
- **Garbage collector:** works, slower than big Go's, and weaker on AVR and WebAssembly.

## Panic recovery: check the version

- The language-features page (last revised for this topic in 2023) says `recover` worked on
  most architectures except WebAssembly, and that runtime panics (nil dereference, divide
  by zero) could not be recovered.
- **TinyGo 0.42** made runtime, map and channel panics recoverable and added `recover` on
  riscv64.
- **TinyGo 0.43** added panic recovery on Xtensa (ESP32) and recovery on WebAssembly
  without the exceptions proposal (reusing the asyncify machinery), and preserves the wasm
  panic traceback. It also added a `-panic-unwind` option.
- So: on 0.43 recovery is broadly available; on older releases do not rely on it. In all
  cases, return errors instead of using panic/recover for control flow.

## Porting existing Go code

- **Serialization:** reflection gaps break `encoding/json` and similar packages. Use a
  package that avoids reflection (often the speed-optimized ones) or write the encoding by
  hand.
- **`reflect.SliceHeader` / `StringHeader`:** TinyGo's versions use `uintptr` for `Len` and
  `Cap` (because AVR has 16-bit pointers while Go requires a 32- or 64-bit `int`), and the
  types are not portable anyway. Use `unsafe.Slice`, `unsafe.SliceData`, `unsafe.String`
  and `unsafe.StringData` instead.

## Interrupts

Usually the `machine` package hides interrupts (for example behind `Pin.SetInterrupt` or
UART receive buffers). When you do work with them directly:

1. the handler is registered with `interrupt.New(irq, handler)`;
2. the peripheral must enable the interrupt source;
3. on chips that have them, set a priority (`SetPriority`);
4. enable the interrupt (`Enable`).

Handler rules from the docs:

- never block: no allocation, no blocking channel operations (use `select` with
  `default`), no `fmt`/printing; prefer an LED for debugging;
- keep it short and push heavy work to normal code (top half / bottom half);
- variables shared with normal code need volatile access (`runtime/volatile`) or atomics;
- `interrupt.New` rejects closures that capture local variables; bind a method with a
  pointer receiver on a global value;
- if an interrupt never fires, check the peripheral's enable bit, `Enable()`, and that the
  IRQ number is the right one; if it fires forever, the chip may require clearing the
  event flag (common on Microchip SAM parts).

## volatile

Go has no `volatile` keyword. Memory-mapped register types in TinyGo's device packages have
`Get`/`Set` methods that already use volatile operations. Use `runtime/volatile` yourself
only for variables shared between an interrupt handler and normal code.

## Differences from big Go worth knowing

- The whole program is compiled at once: more optimization, slower rebuilds for large
  programs.
- Interface method calls are resolved where they are used rather than through
  precomputed tables, which can avoid allocation when an interface value is created.
- Package-level variables are evaluated at compile time where possible (see the memory
  digest).

## Versions (October 2026)

- TinyGo 0.43.0 is current; it builds with Go 1.25 through 1.27. Check the compatibility
  matrix before upgrading Go.
- 0.43 also added, among others: ESP32 PWM via the LEDC peripheral, ESP32xx I2S, several
  ESP32 I2C fixes, USB mass storage on nRF52840, multicore fixes for `-scheduler=cores` on
  RP2040/RP2350, and ESP32-H2, Waveshare RP2040/RP2350-PiZero and Pimoroni Tufty 2350
  targets.
- 0.42 added Go 1.27 support, Bluetooth on ESP32-C3 and ESP32-S3, a UEFI target, and
  several STM32 and Puya PY32F additions.
