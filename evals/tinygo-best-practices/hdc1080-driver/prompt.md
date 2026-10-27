---
tags: [tinygo-best-practices]
max_turns: 40
timeout_seconds: 1500
allowed_tools: [Read, Glob, Grep, Skill, Write, Edit, Bash]
---

I need a TinyGo driver for the TI HDC1080 temperature/humidity sensor so we can upstream it later. Put it in the current directory as a Go module (module example.com/hdc1080) with the driver in ./hdc1080 and an example program in ./examples/basic for a Raspberry Pi Pico (sensor on I2C0, default pins).

What the datasheet says, in short:
- I2C address 0x40. Register pointer is one byte.
- 0x00 temperature, 0x01 humidity (16-bit, MSB first). After writing the pointer byte, the conversion takes up to 6.5 ms for temperature and 6.35 ms for humidity at 14-bit resolution; then read two bytes.
- Temperature in °C = raw / 65536 * 165 - 40. Relative humidity in % = raw / 65536 * 100.
- 0x02 configuration (16-bit). Bit 15 = software reset, bit 12 = mode (0: one measurement per trigger), bit 10 = temperature resolution (0: 14-bit, 1: 11-bit), bits 9:8 = humidity resolution (00: 14-bit, 01: 11-bit, 10: 8-bit).
- 0xFE manufacturer ID = 0x5449, 0xFF device ID = 0x1050.

TinyGo is installed. Tell me what you checked.
