---
type: llm
focus: trace
weight: 1
---

REVIEW.md flags that the battery voltage divides ADC.Get() by 4095 although TinyGo returns 16-bit values (0 to 65535), so the voltage is wrong by about a factor of 16
