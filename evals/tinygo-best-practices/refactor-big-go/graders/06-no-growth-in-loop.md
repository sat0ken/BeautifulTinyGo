---
type: llm
focus: trace
weight: 1
---

The main loop no longer grows a slice (history), writes to a map, or builds strings by
concatenation/Sprintf on every reading; any history is a fixed-size ring buffer.
