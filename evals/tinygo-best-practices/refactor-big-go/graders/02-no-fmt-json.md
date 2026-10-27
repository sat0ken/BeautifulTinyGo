---
type: llm
focus: trace
weight: 1
---

The result does not import fmt or encoding/json; the configuration is a Go struct literal or
constants, and output is produced with println/strconv into a reused buffer or similar.
