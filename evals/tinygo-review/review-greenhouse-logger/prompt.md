---
tags: [tinygo-review]
max_turns: 40
timeout_seconds: 1200
allowed_tools: [Read, Glob, Grep, Skill, Write, Edit, Bash]
---

A contractor wrote this firmware for our greenhouse logger (Raspberry Pi Pico, an Si7021 sensor on I2C0, a 240x240 ST7789, a button on GP14). It works on the bench for a while and then stops responding. Can you review it and tell me what to fix before we deploy ten of them? Write the review to REVIEW.md. Don't change the code.

Input files: go.mod, main.go, si7021/si7021.go
