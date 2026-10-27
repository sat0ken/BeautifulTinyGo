# Changelog

## 0.4.0 (2026-10-27)

- `tinygo-port` also ports ESP-IDF programs and Moddable SDK programs.
  - New `references/espidf-to-tinygo.md`:
    - what disappears without ESP-IDF and FreeRTOS: preemption and the second core, the
      watchdogs (TinyGo disables them at start-up), PSRAM, NVS, sleep modes and OTA;
    - TinyGo 0.43's support per ESP chip;
    - tables for GPIO and interrupts, I2C (new and legacy drivers), SPI, LEDC, ADC, UART,
      esp_timer, FreeRTOS tasks, queues and semaphores, NVS, and Wi-Fi and BLE through
      tinygo.org/x/espradio;
    - a worked example.
  - New `references/moddable-to-tinygo.md`:
    - the XS event loop mapped to the main loop;
    - host providers as the source of pins;
    - ECMA-419 IO and the legacy `pins/*` modules;
    - sensor classes mapped to tinygo.org/x/drivers;
    - Timer, Preference, Poco and Piu, and the network modules;
    - JavaScript number semantics;
    - a worked example.
  - The references record the differences found in TinyGo's source that change values
    without a compile error:
    - TinyGo picks the LEDC duty resolution;
    - the ESP I2C driver does not enable internal pull-ups;
    - the ADC uses a fixed 11 dB attenuation with no calibration;
    - Moddable's SMBus 16-bit reads are little-endian by default;
    - Moddable's SHT3x returns humidity as a fraction, and drivers/sht3x 0.36 checks
      neither the CRC nor bus errors.
  - `SKILL.md`: new sections for ESP-IDF and Moddable programs, and for licensing:
    ESP-IDF is Apache-2.0; the Moddable runtime is LGPL-3.0 and its examples are CC BY 4.0.
  - The checklist and PORTING.md now cover concurrency, configuration sources and
    licenses.
  - Verification:
    - both worked examples build for `esp32c3-generic` and type-check for
      `esp32s3-generic` and `esp32-generic`;
    - a Wi-Fi program using espradio builds for ESP32-C3.
- `tinygo-review`: two new rules, 46 in all.
  - PORT-TASKS: FreeRTOS tasks and JavaScript callbacks mapped onto the cooperative
    scheduler.
  - PORT-PLATFORM: services the original platform provided, replaced or reported.
  - PORT-LICENSE now covers ESP-IDF and Moddable code.
- `arduino-to-tinygo.md`: the ESP chips have four LEDC timers (`PWM0`–`PWM3`).
- No new eval run for this version.

## 0.3.0 (2026-10-27)

- New skill `tinygo-review`, modelled on BeautifulRust's `rust-review`: reviews TinyGo code,
  repositories and diffs without changing them, and writes a prioritized report (Must fix,
  Should fix, Consider, What's already good). Every finding cites `file:line`, a rule ID,
  evidence from tools or "by reading", and a code fix.
- `skills/tinygo-review/references/rules.md`: a catalog of 44 rules (TG- for TinyGo,
  KB- for koebiten, PORT- for ports), each with what to look for, why it matters on a
  microcontroller, how to confirm it, and a source to look up (TinyGo documentation pages,
  `src/machine`, tinygo.org/x/drivers, Go Code Review Comments, koebiten source).
- The review procedure records two limits of `-print-allocs` found while building the eval:
  it does not report allocations inside runtime helpers (string building, `append`
  growth, map inserts), and it can miss slices passed to `drivers.I2C` when the program
  links only one bus implementation, because the interface call is devirtualized.
- New eval case `evals/tinygo-review/review-greenhouse-logger`: Pico firmware and an Si7021
  driver with planted problems (a goroutine that never yields, an unbounded NACK-polling
  loop, an allocating interrupt handler, unbounded heap growth, a 12-bit ADC scale, `fmt`
  in the loop, per-call bus slices, driver-convention issues, per-pixel ST7789 redraws).
  First run (Sonnet 5.5, one run per arm, graders hidden from the agent): 14/14 with the
  skills, 9/14 without. Without the skills the review built nothing, cited no rules,
  missed the per-call bus allocations and put the stuck goroutine below the top tier.
  The judge was lenient on one grader in each arm (fixes shown as code, driver
  conventions), so a strict count is about 13 against 8.

## 0.2.0 (2026-10-26)

Changes from the first eval round (one run per case and arm, Sonnet 5.5; with the skills
45/51 graders passed, without 34/51).

- Eval cases moved from `skills/*/evals` to `evals/<skill>/<case>`. In the first round an
  agent listed a skill folder, found the graders next to it and read them.
- `koebiten-games`: HUD text is formatted into a fixed array and drawn through
  `unsafe.String`, so no frame allocates, even when the score changes. The template's
  `TestFramesDoNotAllocate` covers title, play with a changing score, and game over, and
  fails if text is copied. Snapshots must be retaken after the last change, cover every
  scene, and name which scene each one shows. Reports name the koebiten files read.
- `tinygo-best-practices`: refactors report flash/RAM before and after; display previews
  are re-captured after the last edit; start-up waits are bounded too; fixes are explained
  by their TinyGo mechanism (cooperative scheduler, heap growth, blocking interrupts);
  `unsafe.String` views for formatted text.
- `tinygo-port`: two ways to reconcile 60 FPS games with koebiten's 32 ms tick (two logic
  frames per `Update`, or rescaled values); game ports snapshot every screen of the final
  build.
- Two graders reworded to match the skills: whack-a-gopher's allocation check now asks for
  zero allocations while text changes (not "rebuilt only when it changes"), and
  port-arduboy-game's frame-rate check accepts two logic frames per tick.
- Second round on the three weakest cases, with the graders hidden from the agent:
  refactor-big-go 8/8 vs 5/8, whack-a-gopher 7/8 vs 6/8, port-arduboy-game 7/9 vs 6/9.

## 0.1.0 (2026-10-26)

First version.

- `tinygo-best-practices`: TinyGo rules for microcontrollers (memory, standard library,
  scheduling, interrupts, the machine package, drivers, displays, size, testing) written
  against TinyGo 0.43 and tinygo.org/x/drivers 0.36, with digests of the TinyGo
  documentation and Go idioms in `references/`.
- WebAssembly preview for display programs (`assets/wasm-preview`): a canvas
  `drivers.Displayer`, keyboard buttons, a browser page, and `tools/capture.cjs`, a
  headless Node runner that presses keys and writes PNG snapshots. Works with koebiten.
- `koebiten-games`: making koebiten games, with a template game (`assets/koebiten-template`).
- `tinygo-port`: porting Arduino sketches, libraries and M5Stack Arduino code, and Arduboy
  games to koebiten, with the `arduboy` compatibility package (`assets/arduboy-compat`).
- Six eval cases (`evals/<skill>/<case>`).
