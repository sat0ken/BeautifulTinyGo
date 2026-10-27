---
type: llm
focus: trace
weight: 1
---

REVIEW.md flags that draw clears the whole 240x240 ST7789 with SetPixel one pixel at a time (one SPI transaction per pixel) and recommends FillRectangle/FillScreen or redrawing only what changed
