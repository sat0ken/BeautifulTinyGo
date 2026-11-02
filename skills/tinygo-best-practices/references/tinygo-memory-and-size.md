# Digest: heap allocation, binary size and small-place tips

Sources (TinyGo documentation, paraphrased for this skill):

- Heap allocation: https://tinygo.org/docs/concepts/compiler-internals/heap-allocation/
- Optimizing binaries: https://tinygo.org/docs/guides/optimizing-binaries/
- Tips, Tricks and Gotchas: https://tinygo.org/docs/guides/tips-n-tricks/
- Differences from Go: https://tinygo.org/docs/concepts/compiler-internals/differences-from-go/

## What allocates

TinyGo uses escape analysis to keep values on the stack, but many Go operations still
need the heap. `-print-allocs=.` lists every heap allocation and why; pass a regular
expression to restrict it to your packages (`-print-allocs=main`,
`-print-allocs='mypkg|main'`).

*Observed with TinyGo 0.43 while building this skill:* `-print-allocs` reports the
allocations the escape-analysis pass works on (`make`, `new`, composite literals, escaping
locals, closures). It printed nothing for string concatenation, `[]byte`→`string`
conversion or `append` growth, which allocate inside runtime helpers. `testing.AllocsPerRun`
under `tinygo test` (implemented in 0.43) counted those correctly, so use both.

Operations that allocate unless the compiler proves otherwise:

- taking the address of a local that escapes (stored globally, returned, kept in a struct
  that outlives the call); passing it to a function that only reads it is fine;
- converting between `string` and `[]byte`, except a string-to-bytes conversion whose
  result is never written;
- converting a `byte` or `rune` to a `string`;
- concatenating strings (unless one side is empty);
- storing a value larger than a pointer in an interface;
- closures whose captured variables together are larger than a pointer;
- creating or modifying maps;
- starting a goroutine.

Patterns that avoid them:

- **One buffer, many slices.** Keep a fixed array of the largest size you need in a
  struct and slice it per use; two non-overlapping slices of the same array can be used at
  the same time if you are careful. The Go wiki's "SliceTricks" page has more recipes.
- **Format into fixed storage.** `strconv.AppendInt(buf[:0], n, 10)` instead of
  `strconv.Itoa` or `fmt.Sprintf`.
- **Initialize once.** Build lookup tables, closures and device structs during set-up.

## Globals and flash

Unlike big Go, TinyGo computes package-level variables at compile time whenever it can.
Start-up is faster, initial data takes less flash than the code that would build it,
unused globals disappear, and data that is never written is marked constant so it can stay
in flash instead of RAM. Write tables (fonts, sprites, waveforms) as package-level
composite literals and never mutate them.

*Observed with the TinyGo 0.43.0 release build while updating this skill:* "never written"
is not enough when the table holds slices. Programs that read 8 KB of table data in their
loop put this much in `.data` (RAM; the `data` column of `-size short`):

| The table (8 KB of bytes) | `pico` | `esp32-generic` |
|---|---|---|
| none (the rest of the program) | 124 B | 64 B |
| `var t = []byte{...}` | 124 B | 64 B |
| `var t = []string{...}` (128 strings of 64 bytes) | 124 B | 64 B |
| `//go:embed t.bin` into a `string` | 124 B | 64 B |
| `var t = [][]byte{...}` (128 slices of 64 bytes) | 8,316 B | 8,256 B |
| `var t = []glyph{...}` (128 structs with a 64-byte `[]byte` field) | 8,316 B | 8,256 B |
| tinyfont 0.7.0 `freesans.Bold9pt7b`, drawn with `tinyfont.WriteLine` (no table of our own) | 3,680 B | 3,620 B |

So a table of slices, including tinyfont's fonts, costs its whole size in RAM. In
github.com/sat0ken/tinygo-esp32-cyd five tinyfont fonts took 28 KB of RAM on an ESP32 (of
192 KB); the same glyphs stored as two strings per font (an index and the bitmaps) took
under 1 KB. Keep big tables flat (one `[]byte` and offsets) or make them strings.

Large binary data written as a Go string literal takes four bytes of source per byte
(`\xNN`). `//go:embed` of a binary file into a `string` keeps the data in flash and the
repository small. (A `.gitignore` with `*.bin` for firmware images also ignores `.bin`
assets: name them `.dat`.)

## Flash budget

Images, fonts and level data add up. When the program no longer fits, the link fails with
a message that `.rodata` does not fit in its flash region (TinyGo 0.44 for ESP32, 4 MB of
flash: `section '.rodata' will not fit in region 'DROM'`). Measure with `-size short`, and
leave large assets out on the smaller boards with build tags (the board's tag from its
target JSON) rather than shrinking every board's build.

## Binary size

- The default optimization level already targets size (`-opt=z`).
- **Avoid large packages.** `fmt` is the usual offender; `println` covers most debugging
  and accepts several arguments.
- `-no-debug` strips debug information. It matters for WebAssembly and desktop builds
  (the docs show it removing about two thirds of a small wasm file); firmware images do
  not contain debug information anyway.
- Further switches, each with a cost:
  - `-scheduler=none` removes goroutine support (`go` statements no longer work);
  - `-panic=trap` drops panic messages, which makes failures harder to diagnose;
  - `-gc=leaking` never frees memory, so it suits only short-lived programs;
  - `-opt=s` or `-opt=2` usually grow the binary but occasionally shrink it.
- `-size short` prints code/data/bss and flash/RAM totals; `-size full` breaks them down by
  package.

## Speed

Roughly in order of effect: `-opt=2`; `-gc=leaking` for short programs (notably on
WebAssembly); `-scheduler=none` (helps a little on WebAssembly, which has no native stack
switching); `-panic=trap`.

## Debugging builds

The default size optimization makes stepping through code hard. `-opt=1` disables some
passes, `-opt=0` most of them (some programs stop working at `-opt=0`). Keep debug
information when you plan to use GDB.

## Concurrency on one core

Most microcontroller targets run goroutines cooperatively on a single core. A goroutine
that never blocks or sleeps keeps the CPU forever; call `runtime.Gosched()` in such loops.

## Build-time variables

`-ldflags="-X 'main.version=1.0.0'"` sets a package-level string at build time. The
variable must not have an initializer in the source, or the flag is ignored. Values can
end up in TinyGo's build cache (for example under `~/.cache/tinygo/`), which matters if
you pass secrets this way.
