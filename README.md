# OpenCodeTUI

Standalone dark-mode (purple-shaded) TUI prototype for future Aros integration.

## Run

```bash
cd /home/parmar7f/project/OpenCodeTUI
go mod tidy
go run .
```

## Included UX

- Deep dark background with purple accents
- Three-column layout (agents, stream, task board)
- Simulated `/plan`, `/divide`, `/work` orchestration
- Live stream updates and task state transitions
- Keyboard-first command input

This is intentionally standalone so UI can be iterated rapidly before wiring real Aros engine calls.
