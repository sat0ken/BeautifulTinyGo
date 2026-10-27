---
type: llm
focus: trace
weight: 1
---

waitForSensor no longer spins forever: it yields (time.Sleep or runtime.Gosched) and gives up after
a bounded number of tries with an error or message.
