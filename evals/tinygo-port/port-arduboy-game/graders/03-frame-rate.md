---
type: llm
focus: trace
weight: 1
---

The 60 FPS original is reconciled with koebiten's 32 ms tick in one consistent way: either the
speeds and frame-based timers (everyXFrames(40), the star animation every 8 frames, 2 px basket
steps, star speeds) are rescaled, or every Update runs two original 60 FPS logic frames so the
original values keep their meaning; PORTING.md states which approach was taken.
