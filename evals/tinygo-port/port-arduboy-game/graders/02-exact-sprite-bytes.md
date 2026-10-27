---
type: llm
focus: trace
weight: 1
---

The basket, star and heart arrays are copied byte for byte and drawn with Arduboy semantics:
drawOverwrite for the basket, drawSelfMasked with two animation frames for the star, drawBitmap for
hearts (for example via arduboy.Canvas from the skill's compat package).
