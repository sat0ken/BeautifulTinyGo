---
name: tinygo-review
description: >-
  Review existing TinyGo code (firmware, drivers, koebiten games, ports from Arduino,
  ESP-IDF or Moddable) for what breaks on a microcontroller: hangs, heap growth,
  interrupt misuse, wrong values, size and speed, driver conventions and testability,
  and produce a prioritized report where every finding cites a location, a named rule
  with its source, and a concrete fix. Use this whenever the user asks to review, audit,
  critique, grade or "check" TinyGo code, a repository, a package, a .go file built with
  tinygo, a diff or a pull request, or asks why their board hangs, runs out of memory or
  what to improve, even if they don't say "review". Pairs with tinygo-best-practices
  (fixing and writing code), koebiten-games and tinygo-port.
---

# TinyGo review

Review TinyGo code against the rules the `tinygo-best-practices` skill uses for writing it,
and report what to change. The output is a review, not a rewrite: the reader decides what
to fix. Change the user's files only when they ask for that.

The rule catalog is `references/rules.md` in this skill: every rule has an ID (TG-…, KB-…,
PORT-…), what to look for, why it matters on a microcontroller, how to confirm it, and a
source the reader can look up. Read it before you start, and cite its IDs.

## 1. Decide the scope

- **What to read.** Exactly what the user pointed at: a file, a package, a repository, or
  a diff. For a diff (`git diff`, a pull request), review the changed lines and anything
  they break, not the whole repository.
- **Which boards.** Find every target the code is built for: the Makefile, README, target
  JSON files in `targets/`, build tags on `hw_*`/`board_*` files, and the board files in
  `$(tinygo env TINYGOROOT)/src/machine/`. Say which targets you reviewed for.
- **Which TinyGo.** `tinygo version`, plus what the README or CI pins. Do not recommend
  features the project's TinyGo lacks (for example `testing.AllocsPerRun` needs 0.43,
  recoverable runtime panics need 0.42), and do point out workarounds a newer TinyGo makes
  unnecessary, as a Consider finding.
- **What kind of code.**
  - *Driver or library* meant to be shared or upstreamed: the TG-DRV rules apply strictly.
  - *Application firmware*: stability and memory rules first; driver-style rules only
    where they affect testability.
  - *koebiten game*: add the KB rules. *Port from Arduino, Arduboy, ESP-IDF or Moddable*:
    add the PORT rules and compare with the original if it is available.
- **The project's own rules.** Read CLAUDE.md, AGENTS.md, CONTRIBUTING.md and docs
  describing deliberate choices. They win over this skill's defaults; a documented
  workaround (for a TinyGo bug, a board quirk) is a Consider finding at most.

## 2. Run the tools first, without changing the user's files

Run what is available and fold the results into the report as evidence. Work in a scratch
copy of the module for anything that needs extra files (tests, a fake bus), never in the
user's tree.

```sh
gofmt -l .
go vet ./...                       # packages that build with the normal Go toolchain
tinygo build -o /dev/null -target=<board> -size short <pkg>      # each target in scope
tinygo build -o /dev/null -target=<board> -print-allocs='<regexp>' <pkg>
go test ./...  /  tinygo test ./<pkg>                             # existing tests
```

- **Allocations need two tools.** `-print-allocs` lists what escape analysis moves to the
  heap; it misses allocations inside runtime helpers (string concatenation,
  `string([]byte)`, `strconv.Itoa`, `append` growth, map inserts). For a loop body you
  suspect, write a throwaway `testing.AllocsPerRun` test in the scratch copy and run it
  with `tinygo test` (TinyGo 0.43+). Report the numbers.
- **Interfaces hide driver allocations.** A program that links only one implementation of
  `drivers.I2C`/`drivers.SPI` may show no allocations for slices passed to the bus,
  because TinyGo can devirtualize the call. Review a driver for the general case; if you
  want evidence, link a second implementation (a fake bus that keeps its argument) in the
  scratch copy (TG-ALLOC-BUS).
- **Size:** if a large package (`fmt`, `encoding/json`) is a finding, build once without it
  in the scratch copy and give the before/after flash figure.
- **Display code:** if the project has a WebAssembly build, golden images or a preview,
  run it and look at the result; if it has none, that is a TG-PREVIEW finding, not a
  reason to build one during a review.
- **Unavailable tools:** if TinyGo, a target backend (Xtensa for ESP32 is missing from some
  LLVM builds) or hardware is not available, say so in the report and review by reading.
  Never claim a build or test result you did not run.

## 3. What to look for, in priority order

Work through these in order. The first two groups are where devices die.

**Hangs and crashes on the device** (rules TG-YIELD, TG-BOUNDED-WAIT, TG-ISR,
TG-ISR-SHARED, TG-STACK, TG-RECOVER, TG-ERROR-PATH)
- Goroutines and loops that never yield; the cooperative scheduler then stops everything
  else. Trace when each goroutine first blocks.
- Waits and retries with no bound: polling a status bit, retrying a NACKing read, waiting
  for a pin, at start-up too.
- Interrupt handlers that allocate, print, block or touch slices, maps and strings shared
  with the main loop; shared flags and counters without `sync/atomic`/`runtime/volatile`.
- Large locals and deep call chains in goroutines with small stacks; reliance on
  `recover`; ignored init errors and hot error-retry loops.

**Memory that runs out, and wrong values** (TG-HEAP-GROWTH, TG-ALLOC-*, TG-ADC-SCALE,
TG-INT-WIDTH, TG-CONFIGURE)
- Slices and maps that grow forever; per-iteration allocations in the main loop, frames,
  scans and handlers (strings, `fmt`, boxing, closures, bus slices).
- ADC values treated as 10/12-bit; integer widths and timer wrap-around ported from AVR;
  pins and buses used without explicit configuration.

**Fits and speed** (TG-FMT, TG-REFLECT, TG-FLOAT, TG-SIZE, TG-CONST-DATA,
TG-DISP-SETPIXEL, TG-DISP-REDRAW, TG-DISP-RAM, TG-DISP-DISPLAY)
- `fmt`/`log`/`encoding/json` in firmware; `float64` math on chips without an FPU.
- Per-pixel drawing and full-screen redraws on SPI panels; frame buffers that do not fit.

**Structure and drivers** (TG-MACHINE-EDGE, TG-DRV-*, TG-HW-SOURCE)
- `machine` inside logic or driver packages; functional options; float driver APIs;
  missing `Config`/`DefaultConfig`/validation; sensors without `drivers.Sensor`;
  byte-at-a-time bus traffic; hardware values with no named source.

**Tests, preview and idiom** (TG-TEST-HOST, TG-PREVIEW, Go Code Review Comments)
- Pure logic with no host tests, loop bodies with no allocation test, display code with no
  way to check it off the board; error strings, doc comments, names.

For koebiten games, check KB-TICK, KB-WIDTH, KB-SPLIT, KB-HUD and KB-PIXEL-HELPERS. For
ports, check PORT-BEHAVIOR, PORT-TASKS, PORT-PLATFORM, PORT-FPS and PORT-LICENSE against
the original.

## 4. Write the report

Write it in the user's language. Put it in the file the user names; if they name none, use
the reply for a short review and `REVIEW.md` for a repository-wide one. Use this structure:

~~~markdown
# TinyGo review: <what was reviewed>

<One paragraph: what the code does, the boards and TinyGo version reviewed for, driver /
application / game / port, overall verdict, and the single most important thing to fix.>

**Checked:** <each command run and its result in a few words: builds per target with
flash/RAM, -print-allocs, AllocsPerRun numbers, tests, preview>.
**Not checked:** <what needs the board, or tools that were not available>.

## Must fix
### 1. <Short title> (`main.go:42`)
**Rule:** TG-ISR — interrupt handlers only record the event (TinyGo docs, Interrupts ›
Tips, Tricks and Gotchas)
**Problem:** <what goes wrong on the device, concretely: hang, OOM after N hours, wrong
reading by a factor of 16, …>
**Evidence:** <tool output, a number, or "by reading">
**Fix:**
```go
<the smallest change that fixes it>
```

## Should fix
### 2. ...

## Consider
### 3. ...

## What's already good
- <2 to 4 specific things the code does well>
~~~

How to sort findings:

- **Must fix:** the device hangs, crashes, runs out of memory, reports wrong values, races
  with an interrupt, or does not build for a target in scope; or a driver API mistake that
  becomes a breaking change once upstreamed.
- **Should fix:** it works but costs flash, RAM or speed, allocates in a steady-state path
  without growing, breaks driver conventions, or cannot be tested off the board.
- **Consider:** style, naming, documentation, polish, and workarounds a newer TinyGo makes
  unnecessary.

Rules for each finding:

- **Cite a location** as `file:line` (or a line range, or `whole file`/`whole repository`
  for findings with no single place) and **cite the rule** by ID with its source from
  `references/rules.md`. A reader must be able to look the rule up.
- **Show the fix as code**, only the lines that change. Do not rewrite whole files.
- **Give evidence** when a tool can show it, and say "by reading" when not.
- **Explain by mechanism.** Name what happens on the chip (cooperative scheduler, heap
  fragmentation, SPI transaction per pixel, 16-bit ADC scale), not only the rule.
- **Group repeats.** One finding for "five slice literals passed to the bus in
  si7021.go:61–93", not five findings.
- **Respect intent and hardware facts.** Deliberate, documented choices go under Consider
  with the trade-off. Never tell the user to change a pin, register value or timing
  without a primary source (TG-HW-SOURCE); say what to check instead.
- **Keep it proportionate.** For a small program, five to twelve findings. Do not hide a
  hang among style notes.
- **"What's already good" is required.** It tells the author what to keep.

## 5. Offer the next step

End with one short paragraph:

- offer to apply the Must fix items with the `tinygo-best-practices` skill (or
  `koebiten-games` / `tinygo-port`), one finding per commit if the project uses git;
- suggest the checks that would have caught this automatically, naming two or three
  findings from this review that each would have caught: a Makefile target that builds
  every board with `-size short`, an `AllocsPerRun` test for each loop body, a CI step that
  fails when `-print-allocs` reports a steady-state function, a WebAssembly preview or
  golden images for display code.
