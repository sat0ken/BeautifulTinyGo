---
type: llm
focus: trace
weight: 1
---

The raw ADC value is converted from TinyGo's 0-65535 range to the original 10-bit range (or the
calibration constants are rescaled) so RAW_DRY/RAW_WET keep their meaning, and PORTING.md explains
this.
