---
type: llm
focus: trace
weight: 1
---

Bus reads and writes use fixed-size buffers stored in the Device (or slices of them) rather than
slice literals or local arrays passed to the I2C interface, so a measurement does not allocate.
