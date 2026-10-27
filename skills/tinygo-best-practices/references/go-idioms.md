# Digest: Go idioms, adjusted for microcontrollers

Sources (paraphrased for this skill; both are published by the Go project under
CC BY 4.0, with code under a BSD license):

- Effective Go: https://go.dev/doc/effective_go
- Go Code Review Comments: https://go.dev/wiki/CodeReviewComments

Each item says what the source recommends and, where needed, how it changes under TinyGo.

## Formatting and naming

- `gofmt` decides layout. No exceptions.
- Package names are short, lower case, one word, and not repeated in exported names:
  `st7789.Device`, `st7789.New`, `st7789.Config`, not `st7789.ST7789Device`.
- MixedCaps, never underscores. Initialisms keep one case: `SPI`, `I2C`, `ID`, `URL`
  (`ReadID`, `spiBus`).
- Getters drop `Get`: `d.Temperature()`, not `d.GetTemperature()`. (`machine.Pin.Get` and
  `machine.ADC.Get` are existing APIs; do not copy that naming into new code.)
- Short names for short scopes (`i`, `b`, `d` for a device receiver); longer names for wide
  scopes. Receiver names are short and consistent across methods, never `this`/`self`.

## Comments and documentation

- Every exported identifier gets a doc comment that starts with its name and is a full
  sentence. Package comments start with "Package x ...". In drivers, link the datasheet
  and name the section or table when a value comes from it.
- Comment why, not what. Keep comments brief (the TinyGo and drivers repositories ask for
  short, plain comments in their `AGENTS.md` files).

## Errors

- Return errors; do not panic in library code. Handle or return every error.
- Error strings are lower case and have no trailing punctuation.
- *TinyGo adjustment:* prefer sentinel errors declared once at package level
  (`var errTimeout = errors.New("bme280: timeout")`) over `fmt.Errorf` wrapping in device
  code. `fmt` is large, and building a new error on every failure allocates. Prefix the
  message with the package name so it is recognizable without wrapping.
- Indent the error path, keep the happy path at the left margin; use early returns.

## Interfaces

- Define interfaces where they are used, keep them small, and accept interfaces / return
  concrete types. `drivers.Displayer` (three methods) is the model.
- *TinyGo adjustment:* interface calls are dynamic and slices passed through them escape
  to the heap. Keep interfaces at module boundaries (bus, display, buttons), not in inner
  loops, and pass buffers you own.

## Concurrency

- "Share memory by communicating" and goroutines being cheap are big-Go assumptions.
- *TinyGo adjustment:* goroutines cost a heap-allocated stack each and run cooperatively.
  Use one loop with explicit state machines by default; reach for goroutines and channels
  only for independent blocking work, and never inside drivers or interrupt handlers.

## Data

- Make the zero value useful where you can.
- Prefer arrays and slices of fixed capacity to maps for small, known key sets (key codes,
  register numbers, 6 buttons): faster, smaller, and no allocation.
- Copy slices deliberately; a slice of a buffer aliases it.

## Testing

- Table-driven tests with clear failure messages (`got X, want Y`).
- *TinyGo adjustment:* tests run on the host, so code under test must not import
  `machine`. That is one more reason to keep hardware at the edge.
