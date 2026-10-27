---
type: llm
focus: trace
weight: 1
---

The program does not import fmt; numbers are formatted with strconv.Append* (or equivalent) into
reused buffers or only when values change.
