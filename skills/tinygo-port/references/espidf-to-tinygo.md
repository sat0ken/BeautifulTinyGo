# ESP-IDF → TinyGo mapping

Checked against TinyGo 0.43 (`src/machine/machine_esp32*.go`, `src/runtime/runtime_esp32*.go`,
`targets/esp32*.json`), tinygo.org/x/espradio (main branch, October 2026, after v0.4.0),
tinygo.org/x/drivers 0.36, and the ESP-IDF API reference (v5.x and v6.x: `driver/gpio.h`,
`driver/i2c_master.h`, `driver/spi_master.h`, `driver/ledc.h`, `esp_adc/adc_oneshot.h`,
`driver/uart.h`, `esp_timer.h`, `nvs.h`, FreeRTOS). The example programs at the end build
for `esp32c3-generic` and type-check for `esp32s3-generic` and `esp32-generic`.

Contents: What changes underneath · Check support before porting · Reading an ESP-IDF project · System, errors, logging and time · GPIO and interrupts · I2C · SPI · PWM (LEDC) · ADC · UART · FreeRTOS → goroutines and the main loop · Storage (NVS) · Network and Bluetooth (espradio) · Memory and build configuration · Worked example.

## What changes underneath

ESP-IDF is an RTOS, a driver layer and a build system. A TinyGo program runs bare-metal with
its own runtime, so none of them is there:

| ESP-IDF | TinyGo |
|---|---|
| FreeRTOS, preemptive, tasks on both cores | cooperative `tasks` scheduler on **one core**. A goroutine runs until it blocks (`time.Sleep`, a channel, a mutex, `runtime.Gosched`) |
| task priorities, `xTaskCreatePinnedToCore` | none. Order work explicitly in the main loop |
| task stack depth in `xTaskCreate` | 8 KB per goroutine (`default-stack-size` in `targets/esp32*.json`), heap-allocated |
| task watchdog / interrupt watchdog reboot a hung chip | TinyGo **disables the watchdogs at start-up** (`runtime_esp32*.go`) and has no `machine.Watchdog` on ESP: a hang stays hung |
| PSRAM (`CONFIG_SPIRAM`, `heap_caps_malloc(MALLOC_CAP_SPIRAM)`) | not supported: only internal SRAM. Frame buffers, camera frames and large JSON that lived in PSRAM may not fit |
| partition table, NVS, SPIFFS/LittleFS/FAT partitions, OTA | none. ESP32-C3 has `machine.Flash` (the flash after the program); other ESP chips have no flash storage API in 0.43 |
| `esp_deep_sleep_start`, light sleep, `esp_pm` | not available |
| `idf.py build flash monitor`, `sdkconfig`, Kconfig | `tinygo flash -target=<board> -monitor`; config becomes constants, `-ldflags=-X` strings or build tags |
| `ESP_LOGx` to UART0 with `I (1234) TAG:` prefixes | `println` to the target's serial: USB-Serial-JTAG on C3, S3 and C6, UART0 on ESP32 (`"serial"` in the target JSON) |
| Wi-Fi, Bluetooth (`esp_wifi`, `esp_netif`, lwIP, NimBLE/Bluedroid) | tinygo.org/x/espradio on esp32, esp32c3 and esp32s3 (Wi-Fi STA/AP, ESP-NOW, pure-Go TCP/IP, BLE through tinygo.org/x/bluetooth) |

Building for the Xtensa chips (ESP32, ESP32-S3) needs a TinyGo with the Xtensa LLVM
backend; the release binaries have it. A TinyGo built against upstream LLVM only builds
the RISC-V chips (C3, C6, H2) and stops at compiler-rt for Xtensa, after type checking.

## Check support before porting

TinyGo 0.43, from the build tags of `src/machine/machine_esp32*.go`:

| Peripheral | esp32 | esp32s3 | esp32c3 | esp32c6 |
|---|---|---|---|---|
| GPIO, `Pin.SetInterrupt` (rising, falling, toggle) | ✓ | ✓ | ✓ | ✓ |
| I2C | ✓ | ✓ (`I2C0`, `I2C1`) | ✓ | ✓ |
| SPI | ✓ (`SPI2`, `SPI3`) | ✓ (`SPI0` = FSPI, `SPI1` = HSPI) | ✓ (`SPI2`, alias `SPI0`) | – |
| UART | ✓ | ✓ (`UART0`–`UART2`) | ✓ (`UART0`, `UART1`) | ✓ |
| PWM (LEDC) | ✓ (`PWM0`–`PWM3`) | ✓ | ✓ | – |
| ADC one-shot | ✓ | ✓ | ✓ | ✓ |
| I2S | – | ✓ | ✓ | ✓ |
| USB CDC | – | ✓ | ✓ | ✓ |
| `machine.Flash` | – | – | ✓ | – |
| `machine.GetRNG` | – | ✓ | ✓ | – |
| Wi-Fi, BLE, ESP-NOW (espradio) | ✓ | ✓ | ✓ | – |

Not available on any ESP target: watchdog, PSRAM, sleep modes, RMT, PCNT, MCPWM, TWAI
(CAN), touch pads, DAC, LCD_CAM/camera, SD/MMC host, `esp_restart`. TinyGo exposes every
register through `device/esp`, so a missing peripheral can be written against the
Technical Reference Manual, using ESP-IDF's `hal/` and `soc/` sources as a guide to the
register sequence (Apache-2.0). Tell the user before starting that work.

## Reading an ESP-IDF project

- `main/` and `components/`: the code. `idf_component.yml` and `dependencies.lock`: managed
  components (sensor drivers, `espressif/led_strip`, `esp_lcd_*`): look each up in
  tinygo.org/x/drivers first.
- `sdkconfig`, `sdkconfig.defaults`, `Kconfig.projbuild`: every `CONFIG_*` the code reads
  (pins, Wi-Fi credentials, intervals, log level, `CONFIG_FREERTOS_HZ`). List them; they
  become constants or `-ldflags=-X` values. Never commit credentials into the port.
- `partitions.csv`, NVS keys and SPIFFS paths: what the program stores, and whether that
  data must survive the port (it will not be read by TinyGo).
- IDF version: `dependencies.lock` (`idf: version`), CI files or README. The I2C, ADC and
  timer APIs changed between v4 and v5 (legacy `driver/i2c.h`, `driver/adc.h`); both
  generations are mapped below.
- The chip: `idf.py set-target`, `CONFIG_IDF_TARGET`, or the `sdkconfig` header.

## System, errors, logging and time

| ESP-IDF | TinyGo |
|---|---|
| `void app_main(void)` | `func main()`. It must not return while work remains; `app_main` returning left other tasks running, `main` returning ends the program |
| `esp_err_t` returns | `error` returns. Keep the distinctions the code acts on (`ESP_ERR_TIMEOUT`, `ESP_ERR_NOT_FOUND`) as sentinel errors |
| `ESP_ERROR_CHECK(x)` (abort and reboot) | handle the error: report it and stop, retry with a bound, or return it. Say in PORTING.md that there is no reboot |
| `ESP_LOGI(TAG, "x=%d", x)` | `println("x=", x)` for diagnostics; when the output is parsed, build it with `strconv.Append*` into a fixed buffer and `machine.Serial.Write` (no `fmt`) |
| `printf("%.1f", v)` | integer tenths and `strconv.AppendInt`; record rounding differences |
| `vTaskDelay(pdMS_TO_TICKS(ms))` | `time.Sleep(ms * time.Millisecond)`. A raw tick count is in `CONFIG_FREERTOS_HZ` ticks (100 Hz = 10 ms by default): convert it |
| `esp_timer_get_time()` (µs since boot) | `start := time.Now()` once; `time.Since(start).Microseconds()` |
| `esp_timer_create` + `esp_timer_start_periodic(us)` | a step in the main loop: `if time.Since(last) >= period { last = time.Now(); ... }`. The callback ran in the esp_timer task; here it runs in the loop |
| `esp_random()`, `esp_fill_random()` | `machine.GetRNG()` on C3/S3; elsewhere none |
| `esp_restart()` | none in `machine`; a register write per the TRM, or leave it out and say so |
| `esp_chip_info`, `esp_efuse_mac_get_default` | `device/esp` registers; the MAC address through espradio (`GetHardwareAddr`) |

## GPIO and interrupts

| ESP-IDF | TinyGo |
|---|---|
| `gpio_config(&cfg)` with `.mode = GPIO_MODE_OUTPUT` | `machine.GPIOn.Configure(machine.PinConfig{Mode: machine.PinOutput})` |
| `GPIO_MODE_INPUT` + `pull_up_en` / `pull_down_en` | `machine.PinInputPullup` / `machine.PinInputPulldown` / `machine.PinInput` |
| `GPIO_MODE_OUTPUT_OD`, `GPIO_MODE_INPUT_OUTPUT` | no such `PinConfig` mode on ESP in 0.43; reconfigure between input and output, or set the pad register through `device/esp` |
| `gpio_set_level(n, 1)` / `gpio_get_level(n)` | `pin.High()` / `pin.Set(true)` / `pin.Get()` |
| `GPIO_INTR_POSEDGE` / `NEGEDGE` / `ANYEDGE` | `machine.PinRising` / `PinFalling` / `PinToggle` |
| `GPIO_INTR_LOW_LEVEL` / `HIGH_LEVEL` | no level interrupts: use an edge and read the level in the loop |
| `gpio_install_isr_service(0)` + `gpio_isr_handler_add(n, isr, arg)` | `pin.SetInterrupt(machine.PinFalling, func(p machine.Pin) { ... })`; the pin is the argument, so `arg` is not needed |
| `IRAM_ATTR` handler doing `xQueueSendFromISR` | the handler only records the event (`atomic.Uint32` counter, flag, or a fixed ring buffer); the loop does the work. Rules: tinygo-best-practices §5 |
| GPIO numbers `GPIO_NUM_9` | `machine.GPIO9`; prefer the board's names (`machine.LED`, `machine.SDA_PIN`) when the TinyGo board file has them |

## I2C

ESP-IDF v5.2+ (`driver/i2c_master.h`):

| ESP-IDF | TinyGo |
|---|---|
| `i2c_new_master_bus(&bus_cfg, &bus)` with `.sda_io_num`, `.scl_io_num` | `machine.I2C0.Configure(machine.I2CConfig{SDA: ..., SCL: ..., Frequency: hz})` |
| `i2c_master_bus_add_device(bus, &dev_cfg, &dev)` with `.device_address`, `.scl_speed_hz` | keep the address in the driver's `Device`; the speed is per bus in TinyGo. With devices at different speeds, use the slowest |
| `i2c_master_transmit(dev, w, n, timeout)` | `bus.Tx(addr, w, nil)` |
| `i2c_master_receive(dev, r, n, timeout)` | `bus.Tx(addr, nil, r)` |
| `i2c_master_transmit_receive(dev, w, wn, r, rn, timeout)` | `bus.Tx(addr, w, r)` (write, repeated start, read) |
| `i2c_master_probe(bus, addr, timeout)` | read a known register (an ID register) and check the error |
| `.flags.enable_internal_pullup = true` | TinyGo 0.43's ESP I2C does **not** enable internal pull-ups. Check the board has external ones; say so in PORTING.md |
| `xfer_timeout_ms` | TinyGo's ESP I2C uses a fixed 40 ms timeout per transaction (`machine_esp32xx_i2c.go`) and returns an error; bound the retries around it |

Legacy `driver/i2c.h` (IDF v4 and v5, deprecated since v5.2):

| ESP-IDF | TinyGo |
|---|---|
| `i2c_param_config` + `i2c_driver_install` | `machine.I2C0.Configure(...)` |
| `i2c_master_write_to_device(port, addr, w, n, ticks)` | `bus.Tx(addr, w, nil)` |
| `i2c_master_read_from_device(...)` | `bus.Tx(addr, nil, r)` |
| `i2c_master_write_read_device(...)` | `bus.Tx(addr, w, r)` |
| `i2c_cmd_link_create`, `i2c_master_start`, `i2c_master_write_byte`, `i2c_master_cmd_begin` | collect the bytes the command list writes and reads into one `Tx`; a `i2c_master_start` in the middle is the repeated start of `Tx(addr, w, r)` |

Register reads (`write reg, read n`) become `Tx(addr, regBuf[:1], rx[:n])` with `regBuf` and
`rx` as fields of the driver's `Device`, not slice literals per call.

## SPI

| ESP-IDF | TinyGo |
|---|---|
| `spi_bus_initialize(SPI2_HOST, &buscfg, SPI_DMA_CH_AUTO)` | `machine.SPI0.Configure(machine.SPIConfig{SCK: ..., SDO: ..., SDI: ..., Frequency: hz, Mode: 0})`. Bus names differ by chip (table above). No DMA |
| `spi_bus_add_device(host, &devcfg, &dev)` with `.spics_io_num`, `.clock_speed_hz`, `.mode` | the driver gets the bus and its CS pin and toggles CS itself, like tinygo.org/x/drivers. Leave `SPIConfig.CS` unset (it exists only on C3/S3 and selects one device). Speed and mode are per bus: reconfigure when devices differ |
| `spi_device_transmit(dev, &t)` / `spi_device_polling_transmit` | `cs.Low(); bus.Tx(w, r); cs.High()` |
| `.cmd`, `.addr` phases, `SPI_TRANS_USE_TXDATA` | put the command and address bytes in front of the data in the write buffer |
| `pre_cb` setting a D/C pin (displays) | set the D/C pin before `Tx`; better, use the display's driver in tinygo.org/x/drivers |
| `esp_lcd_panel_*` (ST7789, ILI9341, …) | tinygo.org/x/drivers display packages (`st7789`, `ili9341`, …) |

## PWM (LEDC)

| ESP-IDF | TinyGo |
|---|---|
| `ledc_timer_config` with `.timer_num`, `.freq_hz`, `.duty_resolution` | `machine.PWM0`–`PWM3` are LEDC timers 0–3: `pwm.Configure(machine.PWMConfig{Period: uint64(time.Second) / freq})`. TinyGo **chooses the duty resolution** from the frequency |
| `ledc_channel_config` with `.gpio_num`, `.channel` | `ch, err := pwm.Channel(pin)`; channels come from a shared pool (8 on ESP32/S3, 6 on C3) |
| `ledc_set_duty(mode, ch, d)` + `ledc_update_duty` | `pwm.Set(ch, pwm.Top()*d/maxDuty)`, where `maxDuty` is `(1 << duty_resolution) - 1` from the original: duty values do not carry over |
| `ledc_set_fade_with_time`, `ledc_fade_start` | step the duty in the main loop |
| several pins at the same frequency | one `PWMn`, several channels; a different frequency needs another `PWMn` |

## ADC

| ESP-IDF | TinyGo |
|---|---|
| `adc_oneshot_new_unit` + `adc_oneshot_config_channel` | `machine.InitADC()` once; `a := machine.ADC{Pin: machine.GPIOn}; a.Configure(machine.ADCConfig{})` |
| `adc_oneshot_read(handle, ch, &raw)` (0–4095 at 12 bits) | `a.Get()` returns **16-bit** (0–65520 on ESP chips): `raw12 := a.Get() >> 4` when thresholds were written for 12 bits |
| `.atten = ADC_ATTEN_DB_12` (`ADC_ATTEN_DB_11` in older IDF) | TinyGo fixes the attenuation at 11 dB, the widest range (`machine_esp32*_adc.go`). An original using `DB_0`, `DB_2_5` or `DB_6` reads a narrower voltage range: its thresholds do not carry over, tell the user |
| `adc_cali_*` (eFuse calibration, millivolts) | no calibration: raw values only. Convert with a linear fit measured on the board, and say the result is uncalibrated |
| ADC2 pins with Wi-Fi on | same hardware limit: ADC2 cannot be read while Wi-Fi runs. Use ADC1 pins |

## UART

| ESP-IDF | TinyGo |
|---|---|
| `uart_param_config` + `uart_set_pin` + `uart_driver_install(port, rx_buf, ...)` | `machine.UART1.Configure(machine.UARTConfig{BaudRate: 9600, TX: ..., RX: ...})`; the RX ring buffer is fixed (`machine.NewRingBuffer`) |
| `uart_write_bytes(port, buf, n)` | `uart.Write(buf)` |
| `uart_read_bytes(port, buf, n, ticks)` with a timeout | loop on `uart.Buffered()` / `uart.ReadByte()` with an elapsed-time bound and `time.Sleep` between polls |
| UART events queue (`UART_PATTERN_DET`, break, FIFO overflow) | parse in the loop; there are no events |
| `.data_bits`, `.parity`, `.stop_bits` other than 8N1 | `uart.SetFormat(dataBits, stopBits, parity)` on C3 and C6; ESP32 and S3 have no `SetFormat` in 0.43 (8N1 only) |

## FreeRTOS → goroutines and the main loop

| FreeRTOS | TinyGo |
|---|---|
| `xTaskCreate(fn, "name", stack, arg, prio, &h)` running `for (;;) { work; vTaskDelay(..) }` | a step in the main loop that runs when its period has passed (preferred for short work), or `go fn(arg)` for independent blocking work (network, a long protocol). Priorities do not exist |
| two tasks that each block for a long time | two goroutines that each `time.Sleep`: sleeping yields, so they interleave. A goroutine that loops without blocking starves everything |
| `xQueueCreate(n, size)`, `xQueueSend` / `xQueueReceive` between tasks | a buffered channel `make(chan T, n)`; `select` with `time.After`, or a deadline, for `xQueueReceive(q, &v, ticks)` |
| `xQueueSendFromISR` | not a channel: an atomic counter, a flag, or a fixed ring buffer the loop drains |
| `xSemaphoreCreateMutex`, `xSemaphoreTake/Give` | `sync.Mutex`, or nothing when only one goroutine touches the data |
| binary semaphore given from an ISR | an `atomic.Bool` (or counter) set by the handler and checked by the loop |
| `xEventGroupSetBits` / `xEventGroupWaitBits` | an `atomic.Uint32` bit set checked by the loop, or channels |
| `xTaskNotifyGive` / `ulTaskNotifyTake` | a buffered channel of size 1, or a counter |
| `portENTER_CRITICAL(&mux)` / `taskENTER_CRITICAL` | `state := interrupt.Disable()` … `interrupt.Restore(state)` (`runtime/interrupt`), only around data shared with a handler |
| `vTaskDelete(NULL)` | return from the goroutine |
| `uxTaskGetStackHighWaterMark` | none; keep large buffers out of goroutine stacks (globals or struct fields) |
| work spread over two cores for speed | one core: measure whether the port still meets its timing |

## Storage (NVS)

| ESP-IDF | TinyGo |
|---|---|
| `nvs_flash_init`, `nvs_open("ns", NVS_READWRITE, &h)` | ESP32-C3: `machine.Flash` (`ReadAt`, `WriteAt`, `EraseBlocks`; 4 KB erase blocks, 4-byte writes). Other ESP chips: nothing in 0.43, so the value is not kept across resets |
| `nvs_get_u32` / `nvs_set_u32` + `nvs_commit` | a fixed record (magic, version, fields) read at start; written by erase + write |
| missing key (`ESP_ERR_NVS_NOT_FOUND`) | a record without the magic means "never written": use the default |
| NVS wear levelling | none: one erase per write. Batch writes (save a few seconds after the last change) and record the difference; for files, tinygo.org/x/tinyfs (littlefs) on `machine.Flash` |
| data already in the device's NVS partition | not migrated: a TinyGo image has no partition table and never reads NVS, and flashing it can overwrite that area. Tell the user |

## Network and Bluetooth (espradio)

Module `tinygo.org/x/espradio` on esp32, esp32c3 and esp32s3. Pin a version in `go.mod`;
the API is still moving, so read its `examples/` for the commit you use.

| ESP-IDF | TinyGo |
|---|---|
| `esp_netif_init`, `esp_event_loop_create_default`, `esp_wifi_init`, `esp_wifi_set_mode(WIFI_MODE_STA)`, `esp_wifi_set_config`, `esp_wifi_start`, `esp_wifi_connect`, wait for `IP_EVENT_STA_GOT_IP` | `radio := &link.Esplink{}` (`tinygo.org/x/espradio/netlink`); `netdev.UseNetdev(radio)`; `radio.NetConnect(&nl.ConnectParams{Ssid: ssid, Passphrase: pass, ConnectTimeout: 20 * time.Second})` (DHCP included) |
| `WIFI_EVENT_STA_DISCONNECTED` handler that reconnects | `radio.NetNotify(func(e nl.Event) { ... })` to learn of it; reconnect from the loop with a bounded backoff |
| soft-AP (`WIFI_MODE_AP`) | `radio.NetConnectAP(...)` |
| lwIP sockets (`socket`, `connect`, `send`, `recv`) | Go `net`: `net.Dial("tcp", "host:port")`, `conn.Write`, `conn.Read` |
| `esp_http_client` | `net/http` works but allocates; for long-running firmware prefer espradio's heapless examples (`http-no-allocs`) |
| `esp_http_server` | `httphi` router with `radio.ListenAndServe(router, 80)` (heapless) |
| `esp-mqtt` (`esp_mqtt_client_*`) | github.com/soypat/natiu-mqtt (espradio `examples/mqtt`) |
| `esp_sntp` | espradio `examples/ntp` |
| `esp_now_*` | espradio ESP-NOW API (`examples/espnow`) |
| NimBLE / Bluedroid GATT server or client | tinygo.org/x/bluetooth with espradio. On ESP32, BLE takes 64 KB of RAM at a fixed address |
| `esp_https_ota`, `esp_tls`, mbedTLS | no OTA; check TLS support in espradio before promising HTTPS |
| `sdkconfig` Wi-Fi credentials | `-ldflags="-X main.ssid=... -X main.password=..."` |

The Wi-Fi program in this skill's verification (connect, then `net.Dial` and a write
every 10 s) is about 131 KB of flash and 17 KB of RAM on ESP32-C3.

## Memory and build configuration

| ESP-IDF | TinyGo |
|---|---|
| `malloc`/`heap_caps_malloc` per message | buffers allocated once (struct fields, globals) and reused |
| `MALLOC_CAP_DMA` buffers | ordinary slices; TinyGo's drivers do not use DMA |
| `CONFIG_*` booleans that enable features | build tags (`//go:build feature_x`) or constants |
| per-chip `#if CONFIG_IDF_TARGET_ESP32C3` | files with `//go:build esp32c3` / `esp32s3` / `esp32` |
| `idf.py size` | `tinygo build -size short` (and `-size full`) |
| `CONFIG_LOG_DEFAULT_LEVEL` | a `const debug = false` that guards diagnostic prints |

## Worked example

Original (ESP-IDF v5.2, ESP32-C3): a button interrupt queues the GPIO number, a task
toggles an LED through LEDC, and another task reads an SHT3x every two seconds.

```c
#define BUTTON_GPIO 9
#define LED_GPIO    8
static QueueHandle_t evt_queue;
static i2c_master_dev_handle_t sht;

static void IRAM_ATTR button_isr(void *arg) {
    uint32_t gpio = (uint32_t)arg;
    xQueueSendFromISR(evt_queue, &gpio, NULL);
}

static void button_task(void *arg) {
    uint32_t gpio; bool on = false;
    for (;;) {
        if (xQueueReceive(evt_queue, &gpio, portMAX_DELAY)) {
            on = !on;
            ledc_set_duty(LEDC_LOW_SPEED_MODE, LEDC_CHANNEL_0, on ? 4095 : 0); // 13-bit
            ledc_update_duty(LEDC_LOW_SPEED_MODE, LEDC_CHANNEL_0);
            ESP_LOGI(TAG, "button %lu", gpio);
        }
    }
}

static void sensor_task(void *arg) {
    const uint8_t cmd[2] = {0x2C, 0x06}; uint8_t d[6];
    for (;;) {
        ESP_ERROR_CHECK(i2c_master_transmit(sht, cmd, 2, 100));
        vTaskDelay(pdMS_TO_TICKS(20));
        ESP_ERROR_CHECK(i2c_master_receive(sht, d, 6, 100));
        float t = -45 + 175.0f * ((d[0] << 8) | d[1]) / 65535.0f;
        float rh = 100.0f * ((d[3] << 8) | d[4]) / 65535.0f;
        ESP_LOGI(TAG, "%.1f C %.1f %%", t, rh);
        vTaskDelay(pdMS_TO_TICKS(2000));
    }
}

void app_main(void) {
    /* gpio_config (pull-up, NEGEDGE), ledc_timer_config (5 kHz, 13 bit),
       ledc_channel_config, i2c_new_master_bus (SDA 5, SCL 6, internal pull-ups),
       i2c_master_bus_add_device (0x44, 400 kHz) */
    evt_queue = xQueueCreate(10, sizeof(uint32_t));
    xTaskCreate(button_task, "button", 2048, NULL, 10, NULL);
    xTaskCreate(sensor_task, "sensor", 4096, NULL, 5, NULL);
    gpio_install_isr_service(0);
    gpio_isr_handler_add(BUTTON_GPIO, button_isr, (void *)BUTTON_GPIO);
}
```

TinyGo (builds for `esp32c3-generic`: 17 KB flash, 9.7 KB RAM; no allocation in the loop):

```go
const (
	buttonPin    = machine.GPIO9 // BOOT button, active low (pull-up, NEGEDGE)
	ledPin       = machine.GPIO8
	sdaPin       = machine.GPIO5
	sclPin       = machine.GPIO6
	sensorAddr   = 0x44
	samplePeriod = 2000 * time.Millisecond
	ledFreqHz    = 5000
)

// presses replaces the queue fed by xQueueSendFromISR.
var presses atomic.Uint32

var measureCmd = [2]byte{0x2C, 0x06}

func main() {
	buttonPin.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	if err := buttonPin.SetInterrupt(machine.PinFalling, func(machine.Pin) {
		presses.Add(1)
	}); err != nil {
		fail("button interrupt", err)
	}

	pwm := machine.PWM0 // LEDC timer 0
	if err := pwm.Configure(machine.PWMConfig{Period: uint64(time.Second) / ledFreqHz}); err != nil {
		fail("ledc", err)
	}
	ledCh, err := pwm.Channel(ledPin)
	if err != nil {
		fail("ledc channel", err)
	}

	bus := machine.I2C0 // external pull-ups needed: TinyGo does not enable the internal ones
	if err := bus.Configure(machine.I2CConfig{SDA: sdaPin, SCL: sclPin, Frequency: 400_000}); err != nil {
		fail("i2c", err)
	}

	var (
		ledOn      bool
		seen       uint32
		lastSample = time.Now().Add(-samplePeriod)
		rx         [6]byte
		line       [32]byte
	)
	for {
		// button_task: one iteration per press, like one xQueueReceive each.
		for n := presses.Load(); seen != n; seen++ {
			ledOn = !ledOn
			var duty uint32
			if ledOn {
				duty = pwm.Top() * 4095 / 8191 // 4095 of a 13-bit timer, rescaled
			}
			pwm.Set(ledCh, duty)
			println("button", uint8(buttonPin))
		}

		// sensor_task: every samplePeriod. The 20 ms measurement wait delays
		// button handling by at most 20 ms; the original ran it in parallel.
		if time.Since(lastSample) >= samplePeriod {
			lastSample = time.Now()
			t, rh, err := readSHT3x(bus, rx[:])
			if err != nil {
				println("sensor:", err.Error()) // was ESP_ERROR_CHECK: abort and reboot
			} else {
				b := appendTenths(line[:0], t/100) // "%.1f C %.1f %%" without fmt
				b = append(b, " C "...)
				b = appendTenths(b, rh/10)
				b = append(b, " %\r\n"...)
				machine.Serial.Write(b)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// readSHT3x returns millidegrees Celsius and hundredths of a percent RH.
func readSHT3x(bus *machine.I2C, rx []byte) (milliC, centiRH int32, err error) {
	if err := bus.Tx(sensorAddr, measureCmd[:], nil); err != nil {
		return 0, 0, err
	}
	time.Sleep(20 * time.Millisecond)
	if err := bus.Tx(sensorAddr, nil, rx[:6]); err != nil {
		return 0, 0, err
	}
	rawT := int64(rx[0])<<8 | int64(rx[1])
	rawH := int64(rx[3])<<8 | int64(rx[4])
	return int32(-45000 + 175000*rawT/65535), int32(10000 * rawH / 65535), nil
}
```

(`appendTenths` formats tenths with `strconv.AppendInt`; `fail` prints the error every
second instead of rebooting. The verified program also checks the SHT3x CRC.)

What PORTING.md records for this port:

- `ESP_ERROR_CHECK` rebooted the chip on a sensor error; the port reports it and keeps
  sampling.
- Log lines lose the `I (1234) app:` prefix; `%.1f` rounds where `appendTenths` truncates.
- LEDC duty is rescaled from 13 bits; TinyGo picks 10 bits at 5 kHz.
- I2C internal pull-ups are not enabled: the board needs external ones.
- The two tasks share one loop; the button can wait up to 20 ms behind a measurement.
