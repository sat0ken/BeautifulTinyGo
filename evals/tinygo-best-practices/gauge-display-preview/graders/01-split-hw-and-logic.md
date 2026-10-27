---
type: llm
focus: trace
weight: 1
---

Drawing and gauge logic live in files that do not import "machine"; the ST7789 and button setup is
in a file with a build tag for the board target (for example //go:build waveshare_rp2040_zero), and
a separate file with //go:build tinygo && wasm provides a canvas display and keyboard buttons.
