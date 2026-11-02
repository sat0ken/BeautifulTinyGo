---
name: tinygo-port
description: >-
  Port Arduino sketches and libraries (.ino, Arduino.h, Wire, SPI, Serial, EEPROM,
  Adafruit/SparkFun/M5Stack/M5Unified libraries), ESP-IDF programs (app_main, FreeRTOS
  tasks and queues, gpio, i2c_master, spi_master, ledc, adc_oneshot, uart, esp_timer,
  NVS, esp_wifi, sdkconfig, idf.py components), Moddable SDK programs (JavaScript on XS,
  manifest.json, mcconfig, ECMA-419 embedded:io and embedded:sensor, device host
  providers, pins/*, Timer, Preference, Poco, Piu), Arduboy games (Arduboy2, Sprites,
  ArduboyTones) and C games on a console SDK (PicoLibSDK's PicoPad games) to TinyGo,
  keeping the original behavior on the user's board. Use this whenever the user asks to
  port, convert, translate, rewrite or "move" Arduino, ESP-IDF, FreeRTOS, Moddable,
  Arduboy, PicoLibSDK or other C game code to TinyGo or Go on a microcontroller, or asks
  what such code would look like in TinyGo. Arduboy games land on koebiten. Pairs with
  tinygo-best-practices (target code style) and koebiten-games (the game runtime).
---

# TinyGo port

The goal is the TinyGo program a TinyGo developer would have written for the same
hardware and the same observable behavior. A line-by-line transliteration compiles badly
or not at all and keeps every problem the original had: a sketch's `setup()`/`loop()`,
globals, `delay()` scheduling and `String` concatenation; an ESP-IDF program's task per
concern, queues, `ESP_ERROR_CHECK` and `sdkconfig` plumbing; a Moddable program's
callbacks, objects per sample and template strings. Port the behavior, not the structure.

One exception: a whole program written against a console SDK that runs its own loop
(PicoLibSDK's PicoPad games, and C game SDKs like it). There the loop, its timing and its
random numbers are the behavior, and a game of a thousand lines is easier to get right,
and to check, when it is translated function by function onto a package that offers the
SDK's API in Go. §8 says how.

## 1. Understand the original before writing anything

1. **Read the program and every library or component it uses** that you will replace.
   Find versions if you can: Arduino libraries, ESP-IDF releases (the I2C and ADC APIs
   changed in v5) and Moddable modules change behavior between versions.
2. **Name both boards**: the board and chip the original ran on (Uno, Leonardo/Arduboy,
   ESP32 Arduino core, M5Stack Basic/Core2/CoreS3; for ESP-IDF `CONFIG_IDF_TARGET` in
   `sdkconfig`; for Moddable the `-p` platform such as `esp32/m5stack_cores3`) and the
   TinyGo target. Look the target up in `$(tinygo env TINYGOROOT)/targets/` and its pins in
   `src/machine/board_*.go`.
3. **List the observable behavior**, including the quiet parts where ports go wrong:
   - pins, modes, pull-ups (internal ones the original enabled), active-low buttons,
     debounce timing. For Moddable, the pins live in the board's host provider, not in the
     program;
   - timing: every `delay`, `vTaskDelay`, timer period, `millis()` interval, frame rate,
     timeout, and which work ran concurrently (FreeRTOS tasks, two cores, JS callbacks);
   - serial output byte for byte: baud rate, line endings, number formatting and
     rounding, log prefixes, startup banner;
   - bus traffic: I2C addresses, register writes and their order, byte order of 16-bit
     values, bus speed, SPI mode and speed;
   - analog scaling (10-bit on AVR, 12-bit on the ESP32 Arduino core and ESP-IDF, 0–1023
     or `resolution` bits in Moddable, attenuation and calibration on ESP32);
   - persistence (EEPROM layout, NVS keys, `Preference` domains), sound, what the display
     shows;
   - start-up and error behavior (a missing sensor; `ESP_ERROR_CHECK` rebooting; a
     constructor throwing), and what the watchdog did.
4. **Note what is incidental**, which does not carry over: the `setup`/`loop` split,
   globals that exist because `loop` cannot keep locals, `String`, `F()`/`PROGMEM`,
   `#define` pin numbers, AVR 16-bit `int`, register-level port writes used for speed,
   blocking `delay` used as a scheduler, task-and-queue scaffolding around work that is
   sequential anyway, `esp_err_t` plumbing, objects and closures that only carry state
   between callbacks.
5. **Check TinyGo support for every peripheral and service** the program uses (Wi-Fi,
   BLE, I2S, PWM on that chip, flash storage, PSRAM, deep sleep, watchdog, USB HID,
   touch, RMT, camera). For ESP chips use the per-chip table in
   `references/espidf-to-tinygo.md`. If something is missing on the target, stop and tell
   the user before porting the rest; propose the closest alternative.
6. If the original has a bug or an ambiguity, keep its behavior, flag it in the porting
   notes, and let the user decide.

## 2. Look for drivers before porting libraries

Check the device list (https://tinygo.org/docs/reference/devices/ and the
tinygo.org/x/drivers source) for every sensor, display and chip. Prefer an existing driver
even if its API differs from the original library, ESP-IDF component or Moddable sensor
class; map the calls in the porting notes, and compare what each checks (CRC, bus errors,
units). Port a library only when nothing exists, and then follow the driver rules in
tinygo-best-practices §7 (a `Device`, `Config`, no `machine` import, buffers in the struct,
integer API). Keep the original's license and credit in the new package.

## 3. Capture the behavior in tests first

Most firmware has no tests, so write characterization tests before porting the logic:

- Move pure logic (parsers, state machines, scoring, unit conversions, debouncers) into a
  package without `machine` and test it with `go test` (or `tinygo test`) against the
  values the original produces. Work them out from the C/C++ or JavaScript, or run the
  original logic on the host when it is portable (C with a small harness, JavaScript
  with Node).
- For serial output, write down the exact expected lines and compare.
- For display programs and games, the WebAssembly preview snapshots are the visual tests
  (tinygo-best-practices §9, koebiten-games §7).
- **Compare with the original, frame by frame.** When the original is C or C++ whose
  platform layer can be stubbed (drawing, sound, buttons, time), compile it on the host
  with a stub header, give it the same random generator and seed and a scripted input,
  print a checksum of its state every N frames, and have a Go test compute the same
  checksums from the port. This finds what no screenshot shows (a `float` rounded the
  other way at frame 5,085, arguments evaluated in another order). The recipe and its
  pitfalls are in `references/c-games-to-tinygo.md`.

## 4. Port

Use the mapping tables:

- `references/arduino-to-tinygo.md`: core Arduino API (`pinMode`, `digitalWrite`,
  `analogRead`, `analogWrite`, `millis`, `Serial`, `Wire`, `SPI`, `attachInterrupt`,
  `tone`, `EEPROM`, `String`, `PROGMEM`, `map`, `constrain`, bit macros), library classes
  to driver packages, and M5Stack Arduino / M5Unified code.
- `references/espidf-to-tinygo.md`: what changes without ESP-IDF and FreeRTOS, TinyGo's
  support per ESP chip, GPIO and interrupts, I2C (new and legacy drivers), SPI, LEDC,
  ADC, UART, esp_timer, FreeRTOS tasks/queues/semaphores, NVS, Wi-Fi and BLE through
  espradio, and a worked example.
- `references/moddable-to-tinygo.md`: the event loop, host providers, ECMA-419 IO and
  sensor classes, legacy `pins/*` modules, Timer, Preference, Poco and Piu, network
  modules, JavaScript number semantics, and a worked example.
- `references/arduboy-to-koebiten.md`: Arduboy2, Sprites and ArduboyTones to koebiten,
  using the `assets/arduboy-compat/arduboy` package for pixel-exact bitmaps and sprites.
- `references/c-games-to-tinygo.md`: C programs on a console SDK (PicoLibSDK): when to
  keep the loop, the compatibility package, C semantics to carry over, comparing with the
  original compiled on the host, third-party assets, memory and flash, and a worked
  example of 19 games.

General rules:

- `setup()`, `app_main()` or a Moddable `main.js` top level becomes the start of `main`
  (or a `setup()` returning the hardware handles); `loop()`, task loops and timer
  callbacks become steps of `for { ... }` with an explicit sleep or frame wait.
- Replace `delay()`-, task- and timer-based schedules with a loop that checks elapsed
  time for each step (`time.Since(last) >= period`). Keep one `time.Sleep` per iteration.
  Use a goroutine only for independent blocking work (a network client), never as a
  stand-in for a FreeRTOS priority.
- **Interrupts and callbacks:** an ISR (`attachInterrupt`, `gpio_isr_handler_add`) or an
  edge callback (`onReadable`, `onChanged`) becomes a `SetInterrupt` handler that only
  records the event (an `atomic.Uint32` counter or flag); the loop runs the original
  handler's body once per event, in order. Moddable already ran callbacks outside the
  interrupt, so this keeps its semantics; ESP-IDF's `xQueueSendFromISR` becomes the
  counter or a fixed ring buffer, not a channel.
- Globals that only existed for `loop()`, tasks or callbacks become fields of a state
  struct.
- Keep numeric behavior: use the same integer widths where overflow or wrap-around is part
  of the behavior (an AVR `int` is 16 bits; use `int16`; JavaScript bitwise operators are
  32-bit). Replace floats with integer milli-units where the chip has no FPU, and record
  rounding differences in printed values. From C: `float` stays `float32` (compute
  `sinf`, `atan2f` and the like with `math` in `float64` and convert), and the order in
  which a call's arguments are evaluated is unspecified in C and differs between
  compilers, which matters when two arguments draw random numbers.
- Rescale analog values on purpose: TinyGo's ADC returns 16-bit values everywhere, with no
  calibration, and ESP chips use one fixed attenuation.
- Error codes, `esp_err_t`, `bool` success flags and exceptions become `error` values; a
  missing sensor is an error the program reports, not a silent zero. `ESP_ERROR_CHECK`'s
  reboot and the watchdog are gone (TinyGo disables the ESP watchdogs): decide with the
  user what a failure should do.
- Configuration (`#define`, `sdkconfig`, `manifest.json` `config`, host provider pins)
  becomes named constants with a comment naming where each value came from; credentials go
  in `-ldflags=-X`, never in the source.

## 5. ESP-IDF programs

ESP-IDF is an RTOS, a driver layer and a build system; TinyGo replaces all three:

- **Concurrency:** FreeRTOS is preemptive and uses both cores; TinyGo's ESP scheduler is
  cooperative on one core. List each task, what it waits on and its period; turn short
  periodic tasks into steps of the main loop and keep goroutines for blocking work. If the
  original depended on parallelism for timing (a task that must answer within N ms while
  another blocks), check the worst case in the port and say what changed.
- **Peripherals:** map each driver call with `references/espidf-to-tinygo.md`. Watch for
  the differences it lists: TinyGo picks the LEDC duty resolution (rescale duties with
  `pwm.Top()`), the ESP I2C driver does not enable internal pull-ups, the ADC returns
  16-bit uncalibrated values at a fixed attenuation, SPI chip-select is the driver's job.
- **Missing services:** no PSRAM, NVS (only `machine.Flash` on ESP32-C3), deep sleep,
  watchdog, OTA, RMT, PCNT, TWAI or camera in TinyGo 0.43. Wi-Fi, ESP-NOW and BLE come from
  tinygo.org/x/espradio on ESP32, ESP32-C3 and ESP32-S3. A peripheral TinyGo lacks can be
  written against `device/esp` registers, using ESP-IDF's `hal/` code as a guide; agree
  on that with the user first, it is new driver work.
- **Data on the device:** NVS contents and SPIFFS files written by the IDF firmware are not
  read by TinyGo; flashing may overwrite them. Tell the user before they flash.
- **Toolchain:** ESP32 and ESP32-S3 are Xtensa and need a TinyGo with the Xtensa backend
  (the release binaries have it); C3 and C6 are RISC-V. If you cannot build for the
  user's chip, type-check (the build gets past type checking before the backend error)
  and say the binary was not produced.

## 6. Moddable SDK programs

Moddable runs JavaScript on the XS engine, usually on top of ESP-IDF, so the ESP-IDF
section applies to the chip as well:

- **Find the pins and configuration outside the program:** the host provider
  (`build/devices/<platform>/targets/<board>/host/provider.js`) for `device.pin.*`,
  `device.i2c.default` and friends; `manifest.json` (and the board's) for `config` and
  `defines`. Cite the file next to each constant.
- **The event loop becomes the main loop.** Every callback (`onReadable`, `onChanged`,
  `Timer.repeat`, `Timer.set`, network callbacks) becomes a step that runs when its event
  has been recorded or its time has come, in the original's order.
- **Sensor classes become drivers.** Map `sample()` objects to integer readings, convert
  units (Moddable's SHT3x humidity is a fraction 0–1), keep the original's checks (CRC),
  and turn a failed reading or a throwing constructor into an `error`.
- **JavaScript semantics:** doubles everywhere, 32-bit bitwise operators, typed-array
  wrapping, `toFixed` rounding, SMBus 16-bit reads that are little-endian unless
  `bigEndian` is set, `undefined` for missing values. The reference lists each.
- **UI:** Poco becomes drawing calls on a tinygo.org/x/drivers display (no full frame
  buffer unless it fits) with tinyfont text; Piu has no TinyGo equivalent, so each screen
  becomes a draw function and state. Preview display ports in WebAssembly.
- **Storage:** `Preference` becomes a record in `machine.Flash` where the target has it;
  batch writes, and say what happens on chips without it.

## 7. Arduboy games

Arduboy is a 128×64 one-bit console with a d-pad, A and B: the same shape as koebiten's
screen and keys. So:

- the target runtime is **koebiten** (keyboards with OLEDs, gopher-board, Gopher Badge,
  PyBadge, Wio Terminal, and the browser);
- copy `assets/arduboy-compat/arduboy/` into the project and draw through
  `arduboy.Canvas`: same buffer layout, `drawBitmap`, `drawSlowXYBitmap`, all `Sprites`
  modes, `INVERT`, `getPixel`, lines and circles drawn pixel for pixel like Arduboy2;
  `Flush` it to koebiten's display once per frame;
- **frame rate:** Arduboy games usually run at 60 FPS (`setFrameRate(60)`, 16 ms frames);
  koebiten ticks every 32 ms. Pick one way to reconcile them and state it in the porting
  notes: run two original logic frames per `Update` (keeps every frame count and speed
  exactly as written, the simplest faithful port), or rescale the values (double per-frame
  speeds, halve frame counts). Doing neither makes the game run at half speed.

## 8. C programs on a console SDK

Some C games are complete programs on a vendor SDK: PicoLibSDK's PicoPad games call
`DrawImgPal`, `DispUpdate`, `KeyGet`, `WaitMs` and `RandU16` from their own `while (True)`
loop and keep their state in globals. Restructuring them into update and draw callbacks
breaks the correspondence that lets you check the port line by line, so:

- write (or reuse) a **compatibility package** that offers the SDK's functions with the
  same names and arguments as methods of a console type, on a tinygo.org/x/drivers-style
  display: drawing into a frame buffer with the SDK's clipping rules, the SDK's key scan
  (its repeat and release timings), time through a replaceable clock, and the SDK's random
  generator, so that the same seed gives the same game;
- embed the console in the game's struct, turn the C globals into its fields and the
  functions into methods, and keep the loop;
- make the C code's busy waits yield: `while (KeyGet() == NOKEY) {}` freezes the
  WebAssembly preview, so let `KeyGet`, `KeyPressed` and `Time` sleep a millisecond when
  the program has not slept for a while;
- convert the C image arrays with a generator into strings or embedded files, which stay
  in flash (tinygo-best-practices §3);
- compare the port with the original compiled on the host (§3), and keep the C game's
  quirks unless the user agrees to change them.

`references/c-games-to-tinygo.md` has the details and the numbers from a worked example
(github.com/sat0ken/tinygo-esp32-cyd: 19 PicoLibSDK games on ESP32 boards and in the
browser).

## 9. Licensing

Check the original's license before porting, keep its copyright notice and license in
the port, and put the attribution in PORTING.md and the README:

- **Arduino libraries and Arduboy games:** licenses vary (MIT, BSD, LGPL, GPL, none). If
  there is no license, the code is not free to redistribute: tell the user, and offer a
  private port or contacting the author.
- **ESP-IDF** and Espressif components are Apache-2.0: a port derived from them keeps the
  notice and states the changes. Managed components from the registry carry their own
  licenses: check each.
- **Moddable SDK:** the runtime and modules (including drivers such as the SHT3x sensor
  class) are LGPL-3.0, build tools GPL-3.0, examples CC BY 4.0 (attribution required),
  parts of XS Apache-2.0, some fonts OFL; Moddable also sells a commercial license. Read
  the header of every file you translate. Translating LGPL code into Go most likely makes the port a
  derivative work: keep it under the LGPL (or a compatible license) and say so, or write the driver from
  the datasheet instead and only use the original to compare behavior. Tell the user
  which one you did.
- **Assets are licensed apart from the code.** Permissively licensed games can still
  carry pictures, logos, level data or titles that belong to others (characters from
  films, the logo of a commercial game, levels taken from another game), and an SDK can
  ship a GPL directory next to Unlicense code. Look at the pictures and the data, not only
  the file headers. Leave such assets (or the game) out, or draw replacements at the same
  size and frame count (so hit boxes stay the same) and rename trademarked titles; list
  them in PORTING.md.
- The user's own program, ported for themselves, needs only their own decision; the rules
  above matter when copied third-party code ends up in the port.

## 10. Deliver

1. Build for the user's target with `-size short`; for display programs and games also
   build the WebAssembly preview and look at the snapshots. Script the keys so every
   screen of the original is reached (title, play, game over, menus), take the snapshots
   from the final build, and say which snapshot shows which screen.
2. Run the characterization tests and, for steady-state loops, an allocation check
   (`-print-allocs` and an `AllocsPerRun` test, tinygo-best-practices §2).
3. Write `PORTING.md` next to the code:
   - a table mapping the original's functions, tasks, callbacks and classes to the new
     code;
   - where each pin and configuration value came from;
   - behavior kept on purpose, including bugs kept and why;
   - deliberate differences (frame rate, concurrency, ADC scale, rounding, log format,
     fonts, sound, storage, error handling instead of reboots) and their effect;
   - licenses of the original code used, and how they are honored;
   - what could not be verified without hardware or without a toolchain for the chip.

## 11. Checklist

- [ ] Original behavior listed, including timing, concurrency, serial output and analog
      scaling.
- [ ] Every peripheral and service confirmed supported on the TinyGo target (or the gap
      reported first).
- [ ] Existing tinygo.org/x/drivers packages used where they exist, with differences in
      checks and units recorded.
- [ ] Logic moved into host-testable packages with characterization tests.
- [ ] No `delay`-scheduler, task-per-concern scaffolding, `String`/template-string
      building, global soup or unbounded waits left.
- [ ] Interrupt handlers only record events; callbacks run from the loop in order.
- [ ] Configuration values are named constants with their source; no credentials in code.
- [ ] Arduboy: `arduboy.Canvas` for drawing, 60→31 FPS scaling handled.
- [ ] C programs on an SDK: a compatibility package with the SDK's names, the loop kept,
      C semantics carried over, and the state compared with the original compiled on the
      host (§3, §8).
- [ ] Licenses checked and honored, including third-party pictures, data and titles (§9).
- [ ] Target build passes (or type-checks, with the reason); display programs previewed;
      `PORTING.md` written.
