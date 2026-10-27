---
tags: [tinygo-best-practices]
max_turns: 50
timeout_seconds: 1800
allowed_tools: [Read, Glob, Grep, Skill, Write, Edit, Bash]
---

Make a TinyGo program for my board: a Waveshare RP2040-Zero (target waveshare-rp2040-zero) with an ST7789 240x240 display on SPI1 (reset GPIO9, DC GPIO12, CS GPIO13, backlight GPIO14, rotation 90) and two buttons to ground on GPIO4 and GPIO5.

It should show a temperature gauge: a big number in °C, a horizontal bar that fills from 0 to 40 °C, and a min/max line under it. For now fake the temperature with a slow sine-like wave in integers. Button GPIO4 switches between °C and °F, GPIO5 resets min/max.

Put it in the current directory as a Go module. TinyGo and Node are installed. I want to see it before I flash it.
