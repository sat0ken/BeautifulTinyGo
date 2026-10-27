---
type: llm
focus: trace
weight: 1
---

The driver exposes a Device type created by a constructor that takes a drivers.I2C
(tinygo.org/x/drivers) and has a Configure method that takes a Config struct (or no options) and
returns an error; resolutions are named types or constants rather than raw integers in the public
API.
