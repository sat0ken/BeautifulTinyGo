---
type: llm
focus: trace
weight: 1
---

Temperature and humidity are returned as integers in fixed units (for example milli-degrees Celsius
and hundredths or thousandths of a percent), not float64; conversions use integer arithmetic or keep
any float internal.
