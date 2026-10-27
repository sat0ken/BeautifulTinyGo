---
type: llm
focus: trace
weight: 1
---

REVIEW.md flags that the blink goroutine (main.go, the empty delay loops) never yields, explains that TinyGo schedules goroutines cooperatively so it starves main (or everything else) once it runs, and puts it in its highest-priority category
