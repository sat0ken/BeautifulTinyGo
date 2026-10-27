---
tags: [koebiten-games]
max_turns: 50
timeout_seconds: 1800
allowed_tools: [Read, Glob, Grep, Skill, Write, Edit, Bash]
---

Make a small "whack-a-gopher" game with koebiten for our TinyGo Keeb booth: gophers pop up in one of three holes for a short time, the player hits the matching key, the round lasts 30 seconds and shows the score at the end. It must run on zero-kb02, on my gopher-board (the gopher_board_spi target in koebiten), and in the browser so visitors can try it on a laptop.

Create it in the current directory as a Go module. TinyGo and Node are installed. Tell me which keys to press on each device.
