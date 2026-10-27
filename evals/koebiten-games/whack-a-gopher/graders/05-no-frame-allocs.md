---
type: llm
focus: trace
weight: 1
---

There is a tinygo test that runs Update and Draw under testing.AllocsPerRun and expects zero
allocations, it passes, and it covers play frames in which the score or timer text changes (or
the text is drawn from a fixed buffer that is never reallocated); building HUD strings by
concatenation or strconv.Itoa every frame fails this criterion.
