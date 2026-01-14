# Daily Tasks TUI

Simple Trello-style daily task board in your terminal.

## Run

```bash
go run .
```

This creates `.daily-tasks.json` in `~/Nextcloud`.

## Keys

- `a` add task
- `e` edit task
- `d` delete task
- `space` move between To Do and Done
- `shift+k` move task up
- `shift+j` move task down
- `shift+h` move task left
- `shift+l` move task right
- `t` cycle theme
- `u` undo last change
- `tab` switch column
- `q` quit

## Reset

At startup and once per minute, the app checks the date. If the date changed, all tasks reset to To Do.
