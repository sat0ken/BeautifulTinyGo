# Arduino → TinyGo mapping

Checked against TinyGo 0.43 (`src/machine`, `src/runtime/interrupt`) and
tinygo.org/x/drivers 0.36. Board-specific names (`machine.D2`, `machine.A0`,
`machine.BUTTON_A`, `machine.SPI0_SCK_PIN`) exist only where the board file defines
them: read `$(tinygo env TINYGOROOT)/src/machine/board_<name>.go`.

## Program structure

| Arduino | TinyGo |
|---|---|
| `void setup() { ... }` | start of `main()`, or `setup()` that returns the configured hardware |
| `void loop() { ... }` | `for { ...; time.Sleep(period) }` |
| globals shared by `setup` and `loop` | fields of a state struct created in `main` |
| `delay(ms)` used to schedule several tasks | one loop; each task runs when `time.Since(last) >= period` |
| `#define LED_PIN 13` | `const ledPin = machine.LED` (or the board's name for it) |

## Digital and analog I/O

| Arduino | TinyGo |
|---|---|
| `pinMode(p, OUTPUT)` | `p.Configure(machine.PinConfig{Mode: machine.PinOutput})` |
| `pinMode(p, INPUT)` / `INPUT_PULLUP` / `INPUT_PULLDOWN` | `machine.PinInput` / `PinInputPullup` / `PinInputPulldown` |
| `digitalWrite(p, HIGH)` | `p.High()` or `p.Set(true)` |
| `digitalRead(p)` | `p.Get()` (bool) |
| `analogRead(A0)` | `machine.InitADC()` once; `a := machine.ADC{Pin: machine.ADC0}; a.Configure(machine.ADCConfig{})`; `a.Get()` |
| analog range 0–1023 (AVR) or 0–4095 (ESP32 core) | **always 0–65535** in TinyGo; rescale (`v >> 6` gives 10 bits) |
| `analogWrite(p, 0..255)` | PWM: `pwm.Configure(machine.PWMConfig{Period: ns})`, `ch, _ := pwm.Channel(p)`, `pwm.Set(ch, pwm.Top()*uint32(v)/255)` |
| which PWM drives a pin | chip-specific: RP2040 slices `machine.PWM0`–`PWM7` (GPIO n is on slice `(n/2)%8`); ESP32, ESP32-S3 and ESP32-C3 use LEDC timers `machine.PWM0`–`PWM3` (0.43+); check the chip's `machine_*_pwm.go` |
| `tone(p, f)` / `noTone(p)` | `tinygo.org/x/drivers/tone`: `s, _ := tone.New(pwm, p)`, `s.SetNote(tone.A4)` or `s.SetPeriod(...)`, `s.Stop()` |
| `pulseIn`, `shiftOut`, `shiftIn` | write them with `Get`/`Set` and `time`; drivers has `shifter`/`shiftregister` for common chips |

## Time

| Arduino | TinyGo |
|---|---|
| `delay(ms)` | `time.Sleep(ms * time.Millisecond)` (yields to other goroutines) |
| `delayMicroseconds(us)` | `time.Sleep(us * time.Microsecond)`; for exact short busy-waits `tinygo.org/x/drivers/delay.Sleep` |
| `millis()` / `micros()` | `start := time.Now()` once, then `time.Since(start).Milliseconds()` / `.Microseconds()` |
| `unsigned long` wrap-around arithmetic | `time.Duration` comparisons; if the original relies on 32-bit wrap, use `uint32` deliberately and note it |

## Serial

| Arduino | TinyGo |
|---|---|
| `Serial.begin(115200)` | USB CDC boards: nothing needed. UART: `machine.Serial.Configure(machine.UARTConfig{BaudRate: 115200})` |
| `Serial.print(x)` / `println` | `print(x)` / `println(x)` (built-ins write to `machine.Serial`); `println` separates arguments with spaces |
| exact formatting (`Serial.print(v, HEX)`, fixed decimals) | `strconv.AppendInt(buf[:0], int64(v), 16)` then `machine.Serial.Write(buf)`; avoid `fmt` |
| `Serial.available()` / `Serial.read()` | `machine.Serial.Buffered()` / `machine.Serial.ReadByte()` |
| `Serial1`, `Serial2` | `machine.UART0`, `machine.UART1`, ... per chip; configure TX/RX pins explicitly |

## I2C (Wire) and SPI

| Arduino | TinyGo |
|---|---|
| `Wire.begin()` / `Wire.setClock(400000)` | `machine.I2C0.Configure(machine.I2CConfig{SDA: ..., SCL: ..., Frequency: 400 * machine.KHz})` |
| `beginTransmission(a); write(reg); write(v); endTransmission()` | `i2c.WriteRegister(a, reg, []byte{v})` or `i2c.Tx(uint16(a), w, nil)` |
| `beginTransmission(a); write(reg); endTransmission(false); requestFrom(a, n); read()...` | `i2c.ReadRegister(a, reg, buf[:n])` or `i2c.Tx(uint16(a), w[:1], buf[:n])` |
| `SPI.begin(); SPI.beginTransaction(SPISettings(f, MSBFIRST, SPI_MODE0))` | `machine.SPI0.Configure(machine.SPIConfig{Frequency: f, Mode: 0, SCK: ..., SDO: ..., SDI: ...})` |
| `digitalWrite(cs, LOW); SPI.transfer(buf, n); digitalWrite(cs, HIGH)` | `cs.Low(); spi.Tx(w, r); cs.High()`; one `Tx` for the whole transaction |

Keep `w`/`buf` in a struct or a package-level array; slice literals passed to `Tx` allocate.

## Interrupts and shared state

| Arduino | TinyGo |
|---|---|
| `attachInterrupt(digitalPinToInterrupt(p), isr, FALLING)` | `p.SetInterrupt(machine.PinFalling, func(machine.Pin) { ... })` (`PinRising`, `PinToggle`) |
| `detachInterrupt(...)` | `p.SetInterrupt(0, nil)` |
| `noInterrupts(); ...; interrupts();` | `state := interrupt.Disable(); ...; interrupt.Restore(state)` (`runtime/interrupt`) |
| `volatile` variables shared with an ISR | `sync/atomic` types (`atomic.Uint32`) or `runtime/volatile` |

The ISR rules from tinygo-best-practices §5 apply: no allocation, no blocking, no printing.

## Data, types and strings

| Arduino | TinyGo |
|---|---|
| `byte`, `boolean` | `byte`/`uint8`, `bool` |
| AVR `int` (16-bit), `long` (32-bit), `unsigned long` | `int16`, `int32`, `uint32` when width matters; `int` (32-bit on ARM/ESP) otherwise |
| `float`/`double` (both 32-bit on AVR) | `float32` |
| `String s = "T=" + String(t);` | `buf = strconv.AppendInt(append(buf[:0], "T="...), int64(t), 10)` with a reused `buf` |
| `F("text")`, `PROGMEM` tables, `pgm_read_byte` | string constants and package-level arrays; read them directly (TinyGo keeps constant data in flash) |
| `sprintf`, `dtostrf` | `strconv.AppendInt` / `strconv.AppendFloat` into a fixed buffer |
| `map(x, a, b, c, d)` | `c + (x-a)*(d-c)/(b-a)` in integers (same truncation as Arduino's `long` math) |
| `constrain(x, lo, hi)` | `min(max(x, lo), hi)` |
| `bitRead`, `bitSet`, `bitClear`, `bitWrite`, `_BV(n)` | `x>>n&1`, `x |= 1<<n`, `x &^= 1<<n`, `1<<n` |
| `random(lo, hi)`, `randomSeed(analogRead(A0))` | `math/rand` or a small xorshift in the program state; seed from `machine.GetRNG()` where the chip has one (RP2, ESP32-C3, STM32, nRF), else ADC noise |
| `EEPROM.get/put` | `machine.Flash` (an `io.ReaderAt`/`WriterAt` block device with `EraseBlocks`) on chips that have it, an I2C EEPROM (`at24cx`), or the target's file system; mind erase-block size and wear |

## Libraries

- A C++ library class becomes a Go package with a `Device`: constructor → `New(bus, pins)`,
  `begin()` → `Configure(Config) error`, getters keep their meaning, `bool` success → `error`.
- Adafruit Unified Sensor style (`getEvent`) maps to `drivers.Sensor`: `Update(which)`
  then value methods.
- Check tinygo.org/x/drivers first: `bme280`, `bmp280`, `sht3x`, `ssd1306`, `st7789`,
  `ili9341`, `ws2812`, `mpu6050`, `mpu6886`, `bmi270`, `ft6336`, `axp192`, `pcf8563`, `sx127x`
  and many more exist. List the API differences in `PORTING.md`.

## M5Stack Arduino code (M5Stack, M5Core2, M5Unified libraries)

`M5.begin()` hides power, display, buttons, speaker and IMU set-up. In TinyGo each part is
explicit, so write a `setup()` per model that returns the parts the sketch uses.

| M5 Arduino | TinyGo |
|---|---|
| `M5.begin()` (Basic/Gray) | target `m5stack`: SPI2 + `ili9341.NewSPI(machine.SPI2, machine.LCD_DC_PIN, machine.LCD_SS_PIN, machine.LCD_RST_PIN)`, `Config{Width: 320, Height: 240, DisplayInversion: true}`, `SetRotation(ili9341.Rotation0Mirror)`, backlight `machine.LCD_BL_PIN` high (as in drivers' `examples/ili9341/initdisplay/m5stack.go`) |
| Core2 | target `m5stack-core2`; the AXP192 PMIC (`drivers/axp192`) must power the LCD before it works; touch is `ft6336` |
| CoreS3 / CoreS3 Lite | no built-in target in TinyGo 0.43: write `targets/m5stack-cores3.json` inheriting `esp32s3` with your own build tag. Its AXP2101 PMIC and AW9523 I/O expander (LCD power and reset) have no drivers in drivers 0.36, so set them with raw I2C writes taken from the datasheets or the M5Unified source, and say so in the notes. Display (ILI9342C) works with `ili9341`; touch `ft6336`; IMU `bmi270` + `bmm150` |
| `M5.Lcd.fillScreen(c)` / `fillRect` | `FillScreen` / `FillRectangle` |
| `M5.Lcd.setCursor/setTextSize/print` | `tinyfont.WriteLine` with a chosen font, or `tinyterm` for a scrolling console |
| `M5.Lcd.drawPng/drawJpg` | `tinygo.org/x/drivers/image/png`, `image/jpeg` with a callback that draws blocks |
| `M5.update(); M5.BtnA.wasPressed()` | read `machine.BUTTON_A` (low when pressed) each loop; edge detection plus debounce in your state struct |
| `M5.Speaker.tone(f, ms)` | Basic: DAC/PWM on the speaker pin with `tone`; note the board's amp and pin |
| colours `TFT_RED`, RGB565 values | `color.RGBA{...}`; the driver converts to RGB565 |

Programs that use the display get the WebAssembly preview (tinygo-best-practices §9),
which is the quickest way to compare the port with the tutorial's screenshots.
