---
type: llm
focus: trace
weight: 1
---

go.mod requires github.com/sago35/koebiten with a fixed version, and the agent read koebiten's
source from the module directory (go list -m -f '{{.Dir}}' ...) before relying on its API.
