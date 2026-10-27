---
type: llm
focus: trace
weight: 1
---

Waiting for a conversion uses time.Sleep for the datasheet conversion time (or a bounded retry loop)
and returns an error on bus failure; there is no unbounded busy-wait.
