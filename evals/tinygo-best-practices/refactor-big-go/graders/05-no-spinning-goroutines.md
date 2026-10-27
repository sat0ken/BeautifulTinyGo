---
type: llm
focus: trace
weight: 1
---

The sensor goroutines that loop without sleeping are removed or made to block/sleep; the agent
explains that the scheduler is cooperative and these loops starve the rest of the program.
