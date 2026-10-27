---
tags: [tinygo-port]
max_turns: 50
timeout_seconds: 1800
allowed_tools: [Read, Glob, Grep, Skill, Write, Edit, Bash]
---

We're moving our plant monitor from an Arduino Uno to a Raspberry Pi Pico running TinyGo. Port plant_monitor.ino to an idiomatic TinyGo program in the current directory (Go module example.com/plantmon) that behaves the same. Use GP26 (ADC0) for the soil sensor, GP15 for the LED, GP14 for the button, and I2C0 on GP4/GP5 for the BH1750. Add a PORTING.md for the reviewer.

TinyGo is installed. Input file: plant_monitor.ino
