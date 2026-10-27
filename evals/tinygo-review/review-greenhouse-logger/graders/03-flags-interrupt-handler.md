---
type: llm
focus: trace
weight: 1
---

REVIEW.md flags that the button interrupt callback allocates (append, string concatenation or strconv.Itoa) and prints, and that state shared between the callback and the main loop (presses, pressed or logbook) is not atomic or volatile
