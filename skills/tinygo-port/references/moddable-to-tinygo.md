# Moddable SDK → TinyGo mapping

Checked against the Moddable SDK sources (`documentation/io/io.md`,
`documentation/pins/pins.md`, `documentation/network/network.md`,
`documentation/commodetto/poco.md`, `documentation/files/files.md`, `modules/io`,
`modules/drivers/sensors/sht3x`, `build/devices/esp32/targets/*/host/provider.js`, October
2026), TinyGo 0.43 and tinygo.org/x/drivers 0.36. The worked example at the end builds for
`esp32c3-generic` and type-checks for `esp32s3-generic` and `esp32-generic`. For the chip
side of an ESP32 port (what TinyGo supports per chip, Wi-Fi, storage, no PSRAM), read
`espidf-to-tinygo.md` as well: Moddable on ESP32 runs on top of ESP-IDF.

Contents: What changes underneath · Reading a Moddable project · ECMA-419 IO · Legacy pins modules · Sensor classes (`embedded:sensor/...`) · Timers and time · Storage · Graphics · Network · JavaScript semantics to carry over · Worked example.

## What changes underneath

| Moddable | TinyGo |
|---|---|
| JavaScript on the XS engine, on top of ESP-IDF (ESP32), the Pico SDK, nRF5 SDK or Zephyr | compiled Go, bare-metal |
| one event loop: callbacks (`onReadable`, timers, network) run one at a time, to completion, never inside an interrupt | a main loop that checks each event source in turn; interrupt handlers only record the event and the loop runs the callback body |
| numbers are doubles; bitwise operators work on 32-bit integers | pick integer types and units on purpose (milli-units, tenths); floats only where the chip has an FPU and the values need them |
| objects, strings and `ArrayBuffer`s allocated freely, garbage collected (`creation` sizes in the manifest) | allocate once, reuse buffers; the per-frame and per-sample rules of tinygo-best-practices §2 |
| exceptions (`throw`, a constructor that fails) | `error` returns; a missing device is an error the program reports |
| `trace()` to xsbug or serial | `println`, or `strconv.Append*` into a fixed buffer and `machine.Serial.Write` when the format matters |
| mods (`mcrun`), xsbug, `debugger` | none: one firmware image |
| Workers (separate VMs, `postMessage`) | a goroutine and a channel, on one core |
| `manifest.json` (`include`, `modules`, `resources`, `config`, `defines`, `platforms`) | `go.mod` for modules, constants for `config`, build tags for `platforms` |
| `mcconfig -d -m -p esp32/<board>` | `tinygo flash -target=<board> -monitor` |

## Reading a Moddable project

- **The platform and board**: the `-p` argument in the README or scripts
  (`esp32/m5stack_cores3`, `esp32/moddable_two`, `esp32/esp32c3`, `pico`), and
  `platforms` in `manifest.json`. That names the TinyGo target to look for.
- **Pins come from the host provider**, not the program:
  `build/devices/<platform>/targets/<board>/host/provider.js` defines `device.pin.*`,
  `device.i2c.default` (`data`, `clock`, `port`), `device.spi.default`,
  `device.serial.default`, `device.analog.default`. Copy each value used into a constant
  in the TinyGo program, with that file as the source. The board's `manifest.json` has
  `defines` (for example `i2c.sda_pin`) and `config` (`screen`, `touch`, `rotation`,
  `format`) for the legacy modules.
- **`config`** values read with `import config from "mc/config"` become constants;
  `defines` become constants in the driver's `Config`.
- **Which IO style** the program uses: ECMA-419 (`embedded:io/*`, `device.io.*`,
  `embedded:sensor/*`) or the legacy pins modules (`pins/digital`, `pins/i2c`, …). Both are
  mapped below.
- **What it keeps**: `Preference` domains and keys, `File` paths, resources (images,
  fonts, sounds) listed in `resources`.
- **Licences of everything copied**: see the licensing section of the skill.

## ECMA-419 IO

| Moddable | TinyGo |
|---|---|
| `new Digital({pin, mode: Digital.Output})` (`device.io.Digital`, `embedded:io/digital`) | `pin.Configure(machine.PinConfig{Mode: machine.PinOutput})` |
| `Digital.Input` / `InputPullUp` / `InputPullDown` | `machine.PinInput` / `PinInputPullup` / `PinInputPulldown` |
| `Digital.InputPullUpDown`, `Digital.OutputOpenDrain` | no such mode on most chips: check `src/machine` for the target; on ESP see `espidf-to-tinygo.md` |
| `initialValue: 1` | `pin.Set(true)` right after `Configure` |
| `d.write(v)` / `d.read()` (0 or 1) | `pin.Set(v != 0)` / `pin.Get()` (bool) |
| `edge: Digital.Rising` / `Falling` / `Rising \| Falling` with `onReadable()` | `pin.SetInterrupt(machine.PinRising / PinFalling / PinToggle, handler)`. The handler bumps an `atomic.Uint32`; the loop runs the `onReadable` body once per event. `this.read()` in the body is `pin.Get()` in the loop |
| `DigitalBank({pins: mask, ...})` | one `machine.Pin` per bit; read them together in the loop |
| `new Analog({pin})`, `read()`, `resolution` | `machine.InitADC()`; `a := machine.ADC{Pin: pin}`; `a.Configure(machine.ADCConfig{})`; `a.Get()` is **16-bit** (0–65535; 0–65520 on ESP). `v / (1 << resolution)` becomes `uint32(a.Get()) * scale / 65536` |
| ESP32 `Analog` values (12-bit, calibrated through `adc_cali`, scaled to the attenuation's range) | TinyGo returns uncalibrated raw values: rescale and say thresholds may shift |
| `new PWM({pin, hz})`, `write(v)` with `0 … (1 << resolution) - 1` | `pwm.Configure(machine.PWMConfig{Period: uint64(time.Second) / hz})`; `ch, _ := pwm.Channel(pin)`; `pwm.Set(ch, pwm.Top() * v / ((1 << resolution) - 1))` |
| `new I2C({data, clock, hz, address})` | `bus.Configure(machine.I2CConfig{SDA: data, SCL: clock, Frequency: hz})` once per bus; the address belongs to the driver's `Device` |
| `i2c.write(buf)` / `i2c.read(n or buf)` (stop by default) | `bus.Tx(addr, buf, nil)` / `bus.Tx(addr, nil, buf)` |
| `i2c.write(buf, false)` then `i2c.read(buf)` (no stop: repeated start), or `writeRead(w, r)` | `bus.Tx(addr, w, r)` |
| `read(count)` returning a new `ArrayBuffer` per call | a buffer in the driver struct, sliced per read |
| `new SMBus({...})`: `readUint8(reg)`, `writeUint8(reg, v)`, `readBuffer(reg, buf)`, `writeBuffer(reg, buf)` | `Tx(addr, []byte{reg}, r[:1])`, `Tx(addr, []byte{reg, v}, nil)`, …, with the buffers as struct fields |
| `readUint16(reg)` / `writeUint16(reg, v)` | **little-endian unless `bigEndian` is `true`** (`modules/io/i2c/smbus.js`). Keep the byte order the original used; most sensors are big-endian, so check every call |
| `new SPI({clock, out, in, hz, mode, select})`, `write`, `read`, `transfer`, `flush(deselect)` | `machine.SPIx.Configure(machine.SPIConfig{SCK, SDO, SDI, Frequency, Mode})`; `cs.Low(); bus.Tx(w, r); cs.High()` |
| `new Serial({transmit, receive, baud, port, format, onReadable})` | `machine.UARTn.Configure(machine.UARTConfig{BaudRate, TX, RX})`; poll `Buffered()` / `ReadByte()` in the loop instead of `onReadable` |
| `PulseCount`, `PulseWidth`, `Wakeable Digital`, `AudioOut` | not in TinyGo for ESP; on RP2040 check PIO-based packages. Tell the user before porting |

## Legacy pins modules

| Moddable | TinyGo |
|---|---|
| `Digital.write(pin, v)` / `Digital.read(pin)` (static, `pins/digital`) | configure the pin once at start, then `Set` / `Get` |
| `new Digital({pin, mode})`, `mode(m)` | `Configure`; changing mode is another `Configure` |
| `new Monitor({pin, mode, edge: Monitor.Rising})` + `onChanged` (`pins/digital/monitor`) | `SetInterrupt` + atomic counter + loop, as for `onReadable`. `monitor.rises` / `falls` are the counters themselves: keep one `atomic.Uint32` per edge |
| `Analog.read(channel)` (0–1023; **on ESP32 the argument is the ADC1 channel, not the GPIO**) | find the GPIO for that channel in the chip's datasheet, `machine.ADC{Pin: machine.GPIOn}`; `a.Get() >> 6` gives 0–1023 |
| `new PWM({pin})`, `write(0 … 1023)` | `pwm.Set(ch, pwm.Top() * v / 1023)` |
| `new I2C({sda, scl, address, hz})`, `read(n)`, `write(...bytes, stop)` | as ECMA-419 `I2C` above |
| `new SMBus(...)`: `readByte`, `readWord`, `readBlock`, `writeByte`, `writeWord`, `writeBlock` | as above; `readWord`/`writeWord` are **little-endian** (`pins.md`) |
| `new Servo({pin})`, `write(degrees)`, `writeMicroseconds(us)` | tinygo.org/x/drivers/servo, or PWM at 50 Hz with the same pulse widths |
| `RMT` (`pins/rmt`) | no RMT in TinyGo for ESP; for WS2812 use tinygo.org/x/drivers/ws2812 |

## Sensor classes (`embedded:sensor/...`)

ECMA-419 sensor classes follow one pattern: `new Sensor({sensor: {...device.i2c.default,
io: device.io.I2C}})`, `configure(options)`, `sample()` returning an object such as
`{thermometer: {temperature}, hygrometer: {humidity}}`, and `close()`.

| Moddable | TinyGo |
|---|---|
| the sensor class for a chip (`embedded:sensor/Humidity-Temperature/SHT3x`, `.../Touch/FT6x06`, `IMU/BMI270`) | the tinygo.org/x/drivers package for that chip (`sht3x`, `ft6336`, `bmi270`, …) when it exists; else write a driver per tinygo-best-practices §7 |
| constructor (often a soft reset and checks) | `dev := pkg.New(bus)` + `dev.Configure(cfg)`; return an error where the original threw |
| `configure({...})` | fields of the driver's `Config` |
| `sample()` returning a new object | `dev.Read…()` returning integers, or `Update(drivers.Temperature \| drivers.Humidity)` and getters: no object per sample |
| property missing when a reading failed (`ret.thermometer.temperature` left `undefined` after a bad CRC) | an `error` from the read |
| units: Moddable's SHT3x returns °C as a double and humidity as a **fraction 0–1** | drivers use integer milli-units (`sht3x`: millidegrees, hundredths of a percent). Convert where the original scaled (`humidity * 100`) |
| `onAlert`, `onSample` callbacks | the loop checks the interrupt pin or polls at the original's interval |

Compare what the original checks with what the driver checks. For example Moddable's SHT3x
verifies the CRC of every reading; tinygo.org/x/drivers/sht3x 0.36 checks neither the CRC
nor bus errors, so a missing sensor reads −45 °C. Record that in PORTING.md, or read the
sensor through the bus with the checks, as in `espidf-to-tinygo.md`'s worked example.

Chips without a driver: `axp2101` (power, M5Stack CoreS3) has none in drivers 0.36;
`bm8563` (RTC) is sold as PCF8563-compatible, so try `pcf8563` and compare the register
maps before relying on it.

## Timers and time

| Moddable | TinyGo |
|---|---|
| `Timer.repeat(cb, ms)` | a step in the main loop: `if time.Since(last) >= period { last = time.Now(); ... }` |
| `Timer.set(cb, ms)` (once) | a deadline field: `if !deadline.IsZero() && time.Now().After(deadline) { deadline = time.Time{}; ... }` |
| `Timer.set(cb, first, repeat)` / `Timer.schedule(id, first, repeat)` | a deadline for the first run, then a period |
| `Timer.clear(id)` | clear the deadline or a `running` flag |
| `Timer.set(cb, 0)` (run soon, after the current callback) | a `pending` flag handled later in the same loop iteration |
| `Timer.delay(ms)` (blocks the VM) | `time.Sleep(ms * time.Millisecond)`, which yields to other goroutines |
| `Date.now()` | `time.Now()` (no wall-clock time until set from NTP or an RTC) |
| several timers firing in the same tick | the loop runs due steps in a fixed order; if the original's order mattered, keep it and say so |

## Storage

| Moddable | TinyGo |
|---|---|
| `Preference.get(domain, key)` / `set` / `delete` (ESP32: ESP-IDF NVS) | a fixed record in `machine.Flash` (ESP32-C3, RP2040/RP2350, nRF, SAMD21/51, STM32 in TinyGo 0.43), read at start and written on change; no flash API on ESP32 and ESP32-S3 |
| `Preference.get` returning `undefined` | no valid record (magic number missing): use the default |
| a `set` on every change | batch writes (a few seconds after the last change): each write erases a flash block. State the trade-off in PORTING.md |
| `File`, `Iterator` (SPIFFS or LittleFS) | tinygo.org/x/tinyfs (littlefs) on `machine.Flash`, or a fixed layout |
| `Resource` (images, fonts, data compiled into the image by mcconfig) | `//go:embed` into a `[]byte` or `string` (TinyGo supports it), after converting images to the display's pixel format at build time; or generated Go constants |

## Graphics

| Moddable | TinyGo |
|---|---|
| `screen` from the board setup (`config.screen: "ili9341"`, `rotation`, `format`) | the display driver from tinygo.org/x/drivers (`ili9341`, `st7789`, `gc9a01`, …) configured with the same pins and rotation |
| Commodetto `Poco`: `begin(x, y, w, h)` … `end()` redraws a dirty area in strips, without a full frame buffer | draw only what changed straight to the display: `FillRectangle` for rectangles, `DrawRGBBitmap8` for images. A full frame buffer usually does not fit (tinygo-best-practices §8) |
| `poco.makeColor(r, g, b)` | `color.RGBA{R: r, G: g, B: b, A: 255}` |
| `poco.fillRectangle(c, x, y, w, h)` | `display.FillRectangle(x, y, w, h, c)` |
| `poco.drawText(text, font, c, x, y)`, `getTextWidth` | `tinyfont.WriteLine(&display, &font, x, y, text, c)` with `y` at the baseline, `tinyfont.LineWidth` |
| BMF fonts (`.fnt` + `.png`, compressed by mcconfig) | tinyfont fonts; generate others with `tinyfontgen` from BDF/TTF. Check the font's licence (some Moddable fonts are OFL) |
| `drawBitmap`, `drawGray`, `drawMasked` with resources | bitmaps converted at build time; transparency drawn by skipping pixels or with a mask |
| `clip`, `origin` | offsets and bounds in your draw functions |
| Piu (`Application`, `Container`, `Label`, `Skin`, `Style`, `Behavior`, `onTouchBegan`, `onTimeChanged`) | no retained UI toolkit: one draw function per screen, state structs, touch events polled from the touch driver, redraw of changed areas only |
| touch (`embedded:sensor/Touch/FT6x06`) | `tinygo.org/x/drivers/ft6336`, the closest driver in the family: compare the registers the original uses |

Check display programs in the WebAssembly preview (tinygo-best-practices §9) and compare
with screenshots of the Moddable simulator (`mcconfig -d -m -p mac`/`win`/`lin`) when the
user can provide them.

## Network

On ESP32-family boards use tinygo.org/x/espradio (see `espidf-to-tinygo.md`, "Network and
Bluetooth").

| Moddable | TinyGo |
|---|---|
| `new WiFi({ssid, password}, msg => …)` with `WiFi.connected`, `WiFi.gotIP`, `WiFi.disconnected` | `radio.NetConnect(&nl.ConnectParams{Ssid, Passphrase, ConnectTimeout})` blocks until DHCP is done; `radio.NetNotify` for disconnects; reconnect from the loop with a bound |
| `Net.get("IP")`, `Net.get("MAC")` | espradio's netdev / `GetHardwareAddr` |
| `Socket`, `Listener` (`socket`), `embedded:io/socket/tcp` / `udp` | `net.Dial`, `net.Listen` |
| `Request` (`http`) with callbacks per header and body chunk | `net/http` (allocates) or a heapless client; read the body into a fixed buffer |
| `Server` (`http`) | `httphi` with `radio.ListenAndServe` |
| `Client` (`mqtt`) | github.com/soypat/natiu-mqtt (espradio `examples/mqtt`) |
| `SNTP` (`sntp`) | espradio `examples/ntp` |
| `WebSocket`, `MDNS`, `DNSServer`, TLS (`SecureSocket`) | check espradio and lneto for support before promising them |
| `BLEServer`, `BLEClient` (`documentation/network/ble`) | tinygo.org/x/bluetooth |

## JavaScript semantics to carry over

- **Numbers**: `a / b` is a float. `Math.floor`, `Math.round`, `toFixed(n)` round in
  specific ways: integer arithmetic plus explicit rounding, and note where the output
  changes (`toFixed(1)` rounds; truncation does not).
- **Bitwise operators** convert to 32-bit integers: `x | 0` truncates toward zero, `>>`
  is signed `int32`, `>>>` is unsigned `uint32`. `(value << 8) | low` from a buffer can
  go negative when the top bit is set; use `uint16`/`int16` on purpose.
- **Typed arrays** wrap: a `Uint8Array` store keeps the low 8 bits, like a Go `byte`
  conversion. `Int16Array` reads are signed.
- **`undefined` and `??`** for missing values: a `(value, ok)` pair, an error, or a
  documented default.
- **Template literals and string concatenation** allocate: build output in a fixed
  buffer.
- **Closures over loop state** (timers capturing variables): fields of a state struct.
- **`async`/`await` and Promises**: sequential code in the main loop or a goroutine,
  with explicit timeouts.

## Worked example

Original (Moddable, `mcconfig -d -m -p esp32/esp32c3`): the button counts presses into a
`Preference` and drives an LED; an SHT3x is sampled every two seconds. Pins come from
`build/devices/esp32/targets/esp32c3/host/provider.js` (`button: 9`, `led: 8`, I2C
`data: 2`, `clock: 3`).

```js
import Timer from "timer";
import Preference from "preference";
import SHT3x from "embedded:sensor/Humidity-Temperature/SHT3x";

const Digital = device.io.Digital;
const led = new Digital({pin: device.pin.led, mode: Digital.Output});
let count = Preference.get("app", "count") ?? 0;

new Digital({
	pin: device.pin.button,
	mode: Digital.InputPullUp,
	edge: Digital.Falling,
	onReadable() {
		count++;
		Preference.set("app", "count", count);
		led.write(count & 1);
		trace(`count ${count}\n`);
	}
});

const sensor = new SHT3x({sensor: {...device.i2c.default, io: device.io.I2C}});
Timer.repeat(() => {
	const {thermometer, hygrometer} = sensor.sample();
	trace(`${thermometer.temperature.toFixed(1)} C ${(hygrometer.humidity * 100).toFixed(1)} %\n`);
}, 2000);
```

TinyGo, `main.go` (builds for `esp32c3-generic`: 16.6 KB flash, 9.7 KB RAM; no allocation
in the loop):

```go
const (
	buttonPin    = machine.GPIO9 // device.pin.button
	ledPin       = machine.GPIO8 // device.pin.led
	sdaPin       = machine.GPIO2 // device.i2c.default.data
	sclPin       = machine.GPIO3 // device.i2c.default.clock
	samplePeriod = 2000 * time.Millisecond
	saveDelay    = 5 * time.Second // Preference.set on every press, batched
)

// falls counts falling edges. On XS, onReadable runs later on the event loop,
// not in the interrupt; the main loop below plays that role.
var falls atomic.Uint32

func main() {
	ledPin.Configure(machine.PinConfig{Mode: machine.PinOutput})
	buttonPin.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	if err := buttonPin.SetInterrupt(machine.PinFalling, func(machine.Pin) {
		falls.Add(1)
	}); err != nil {
		fail("button", err)
	}

	bus := machine.I2C0
	if err := bus.Configure(machine.I2CConfig{SDA: sdaPin, SCL: sclPin, Frequency: 400_000}); err != nil {
		fail("i2c", err)
	}
	sensor := sht3x.New(bus) // address 0x44, like the sensor class default

	count, err := loadCount() // Preference.get("app", "count") ?? 0
	if err != nil {
		println("preferences:", err.Error())
	}

	var (
		seen       uint32
		dirty      bool
		changedAt  time.Time
		lastSample = time.Now()
		line       [32]byte
	)
	for {
		// onReadable(): one call per edge, in order.
		for n := falls.Load(); seen != n; seen++ {
			count++
			ledPin.Set(count&1 != 0) // led.write(count & 1)
			println("count", count)  // trace(`count ${count}\n`)
			dirty, changedAt = true, time.Now()
		}
		if dirty && time.Since(changedAt) >= saveDelay {
			if err := saveCount(count); err != nil {
				println("preferences:", err.Error())
			}
			dirty = false
		}

		// Timer.repeat(() => { ... sensor.sample() ... }, 2000)
		if time.Since(lastSample) >= samplePeriod {
			lastSample = time.Now()
			milliC, centiRH, err := sensor.ReadTemperatureHumidity()
			if err != nil {
				println("sht3x:", err.Error())
			} else {
				// humidity is a fraction (0–1) in the sensor class and
				// hundredths of a percent in the driver.
				b := appendTenths(line[:0], milliC/100)
				b = append(b, " C "...)
				b = appendTenths(b, int32(centiRH)/10)
				b = append(b, " %\n"...)
				machine.Serial.Write(b)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
}
```

`prefs_flash.go` (`//go:build esp32c3`) keeps the count in `machine.Flash`; a
`prefs_none.go` (`//go:build !esp32c3`) returns an error, so the program still builds for
ESP32 and ESP32-S3 and says the count is not kept:

```go
//go:build esp32c3

package main

import (
	"encoding/binary"
	"machine"
)

const prefMagic = 0x50524546 // "PREF"

func loadCount() (uint32, error) {
	var rec [8]byte
	if _, err := machine.Flash.ReadAt(rec[:], 0); err != nil {
		return 0, err
	}
	if binary.LittleEndian.Uint32(rec[0:4]) != prefMagic {
		return 0, nil // never written: Preference.get returned undefined
	}
	return binary.LittleEndian.Uint32(rec[4:8]), nil
}

func saveCount(n uint32) error {
	var rec [8]byte
	binary.LittleEndian.PutUint32(rec[0:4], prefMagic)
	binary.LittleEndian.PutUint32(rec[4:8], n)
	if err := machine.Flash.EraseBlocks(0, 1); err != nil {
		return err
	}
	_, err := machine.Flash.WriteAt(rec[:], 0)
	return err
}
```

(`appendTenths` and `fail` are the same helpers as in `espidf-to-tinygo.md`.)

What PORTING.md records for this port:

- Preferences are saved 5 s after the last press instead of on every press, to spare the
  flash; a reset within those 5 s loses the last presses. On ESP32 and ESP32-S3 the count
  is not kept at all (no flash API in TinyGo 0.43). The count stored by the Moddable
  firmware in NVS is not carried over.
- `toFixed(1)` rounds; the port truncates the last digit.
- The I2C bus runs at 400 kHz; Moddable's SHT3x asks for 1 MHz unless the provider sets
  `hz`.
- drivers/sht3x 0.36 does not check the CRC or bus errors, which Moddable's SHT3x does:
  a missing sensor prints −45.0 C instead of an error.
