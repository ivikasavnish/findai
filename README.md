# findai

Drop-in `find` replacement with natural-language mode.

```
$ find -ai "find folder named apiorm"
$ find . -type d -name apiorm
./Desktop/apiorm
```

- `find -ai "..."` — AI translates the request into find arguments (JSON array), prints the command, runs `/usr/bin/find` directly (no shell).
- Asks before running anything with `-delete`, `-exec`, `-execdir`, `-ok`, `-okdir`, `-fprint*`, `-fls`. Pass `-y` to skip.
- Without `-ai`, execs `/usr/bin/find` unchanged — scripts keep working.

## Install

```
go build -buildvcs=false -o ~/.local/bin/find .
```

`~/.local/bin` must come before `/usr/bin` in `PATH`. Prefer not to shadow `find`? Build as `findai` instead.

## Backends

| Env | Default | |
|-----|---------|-|
| `FINDAI_BACKEND` | `claude` | `claude` uses `claude -p`; `ollama` uses local Ollama |
| `FINDAI_MODEL` | `haiku` / `qwen2.5-coder` | model per backend |
| `OLLAMA_HOST` | `localhost:11434` | |

```
FINDAI_BACKEND=ollama find -ai "go files changed in last 2 days, skip vendor"
```

## License

MIT
