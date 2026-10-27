---
type: llm
focus: trace
weight: 1
---

ADC readings are scaled from 65535 (TinyGo's 16-bit ADC range), not 1023, and the answer points out
this bug.
