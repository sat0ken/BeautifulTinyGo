---
type: llm
focus: trace
weight: 1
---

REVIEW.md flags that the si7021 driver passes slice literals or freshly made buffers to the drivers.I2C interface on every call, which allocate, and recommends fixed buffers kept in the Device struct
