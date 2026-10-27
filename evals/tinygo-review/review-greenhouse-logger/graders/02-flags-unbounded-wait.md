---
type: llm
focus: trace
weight: 1
---

REVIEW.md flags the loop in si7021 ReadHumidity that retries the read until it stops failing, with no bound and no sleep or yield, as a hang when the sensor never answers, in its highest-priority category
