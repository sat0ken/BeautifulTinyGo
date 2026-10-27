# Digest: TinyGo "Driver Design and Development"

Source: https://tinygo.org/docs/guides/driver-design/ (TinyGo documentation, page last
changed April 2026). This is a paraphrased summary written for this skill; read the
original for its full examples and reasoning.

The guide collects advice for drivers that run under TinyGo and under the normal Go
toolchain. It notes that most of it applies to driver code in any language.

## General rules

- **Prefer integers to floating point.** Peripherals usually report integers, and most
  small CPUs either lack floating-point hardware or run it much more slowly than integer
  code. Expose integer results in the public API (for example milli-units). If a float
  is easier inside the driver, keep it internal so it can be removed later without
  breaking users. The background discussion is drivers PR #345.
- **Avoid heap allocation** (see below).
- **Avoid concurrency inside drivers.** No goroutines, channels or `select`. Concurrent
  code is hard to write and much harder to debug on bare metal. Even ports of
  asynchronous reference drivers are made single-threaded; the CYW43439 Wi-Fi driver,
  ported from an async Rust driver, is the guide's example of a successful complex port.
  Make the API safe to call from several goroutines where it is cheap (a mutex), and
  leave the rest for later.
- **Sensors follow the `drivers.Sensor` interface.** The value-returning side of that API
  was still evolving when the guide was written.

## Shape of a peripheral driver

- **One package per peripheral**, named after the part (`mpu6050`). A family that shares
  logic gets one package with the varying character replaced by `x` (`as560x`).
- **A `Device` type is the handle**, and all interaction goes through its methods. It
  stores what the hardware needs: the bus (`drivers.I2C`, `drivers.SPI`) and pins.
- **Do not import `machine`.** People also use the drivers with the normal Go toolchain,
  where `machine` does not exist. Abstract pins as functions: an output is
  `func(level bool)`, an input is `func() bool`.
- **A `Config` struct** for devices with many options:
  - give each register-backed option its own named type, with exported constants that
    use the datasheet's values;
  - do not use functional options or option interfaces: they cost speed, memory and
    binary size, and the hardware they describe will never change;
  - provide a package-level default configuration, optionally with a parameter or two for
    the settings people change most;
  - when options depend on each other, add a method on `Config` that sets them
    consistently, and a validation method (returning `error`) that `Configure` also calls.
    Readers learn how the device works from it.
- The guide points at `pca9685`, `ndir`, `lsm6ds3` and `l3gd20` in tinygo.org/x/drivers as
  drivers that follow these rules (the last two still use an older I2C API that may
  allocate).

## Constraining memory

Heap allocations are allowed in TinyGo, but frequent ones fragment memory and eventually
crash long-running programs.

- **Why bus calls allocate:** a slice holds a pointer. When a slice is passed to a method
  of an interface (such as `drivers.SPI.Tx`), the compiler cannot see the implementation,
  assumes the pointer may be kept, and moves the backing array to the heap. A
  read-register helper that builds `[]byte{addr, 0}` and a local `[2]byte` result buffer
  can therefore allocate twice per call.
- **Fix: put the buffers in the device struct** and pass slices of them. The `Device`
  itself is usually heap-allocated once, which is fine: the goal is no allocations after
  initialization, not none at all. The guide cites NASA's "Power of 10" rules, which
  also allow allocation only during start-up.
- **Variable-length transfers:** pick a maximum transaction size, keep read and write
  arrays of that size in the struct, copy the caller's data in (first byte reserved for
  the register address) and slice to the needed length.
- Small arrays used only locally can stay on the stack; TinyGo has a maximum stack object
  size (around 256 bytes at the time of writing).

## Devices with several bus types

When one part speaks both SPI and I2C, a shared unexported core type with protocol-specific
wrapper types (`DeviceSPI`, `DeviceI2C`) often works best. When the logic is too complex
for that, one `Device` with an internal bus abstraction and two constructors
(`NewI2C`, `NewSPI`) is acceptable.

For sensors, separate reading the hardware (`Update`) from returning values
(`Pressure()`, `Temperature()`): an update can be expensive and may fill several values
at once, and the application decides when to pay for it.

## Hot loops

Busy-waiting for a status bit is normal in drivers, and dangerous when it can run for a
long time or forever.

- Measure first: count iterations and elapsed time, and print after the loop, not inside.
- **Yield inside the loop** (`runtime.Gosched()` for waits under about a millisecond,
  `time.Sleep` for longer ones), and **give every loop a fixed bound** after which it
  returns an error ("all loops must have fixed bounds" is another Power of 10 rule).
- Choose the bound from measurements: roughly twice the observed worst case, and never
  fewer than about ten retries for a fast check. Measure at the fastest bus speed; faster
  CPUs need more iterations.
- When the device is known to take a long time, sleep for the typical duration first and
  only then start polling.
- Put the retry counter check before the readiness check in the loop condition, so a
  success on the last try is not reported as a failure.

## Fewer, larger I/O calls

Each method call through a bus interface is a dynamic call, and toggling a data/command pin
that is already in the right state does nothing useful. Send a command byte once, then all
of its data bytes in one transaction. To keep that allocation-free, copy short constant
sequences into a fixed buffer in the device and send a slice of it.
