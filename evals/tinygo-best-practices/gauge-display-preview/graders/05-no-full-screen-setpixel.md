---
type: llm
focus: trace
weight: 1
---

The program does not repaint the whole screen pixel by pixel every frame: it uses FillRectangle or
similar block operations and redraws only changed regions (or explains why a full redraw is cheap).
