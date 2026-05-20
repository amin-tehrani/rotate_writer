# rotate_writer

> A Go package that automatically rotates any `io.WriteCloser` based on custom conditions — size, time, or arbitrary logic.

[![Go](https://img.shields.io/badge/Go-1.21%2B-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![Go Reference](https://pkg.go.dev/badge/github.com/amin-tehrani/rotate_writer.svg)](https://pkg.go.dev/github.com/amin-tehrani/rotate_writer)
[![License: MIT](https://img.shields.io/badge/License-MIT-green?style=flat-square)](LICENSE)

The canonical use case is log file rotation, but the abstraction is generic: any writer (file, buffer, network stream) can be rotated on any condition you define.

---

## Versions

| Version | Module path | API style |
|---|---|---|
| **v2** (current) | `github.com/amin-tehrani/rotate_writer/v2` | Rule-based with options pattern |
| v1 | `github.com/amin-tehrani/rotate_writer/v1` | Function-based condition (DEPRECATED) |

---

## v2 (current)

### Installation

```bash
go get github.com/amin-tehrani/rotate_writer/v2
```

### Rotate log files by size

```go
import (
    rotate_writer "github.com/amin-tehrani/rotate_writer/v2"
    "github.com/amin-tehrani/rotate_writer/v2/rule"
)

// Rotate to a new file every 10 MB, named: app-001.log, app-002.log, ...
r, err := rule.NewTemplateFileRotateRule(
    "app-{{printf \"%03d\" .Count}}.log",
    rule.WithMaxSize(10 * 1024 * 1024),
)

rw, err := rotate_writer.NewRotateWriter(r, err)
if err != nil {
    log.Fatal(err)
}
defer rw.Close()

log.SetOutput(rw)
log.Println("this goes into app-001.log, rotating at 10 MB")
```

### Rotate by time

```go
r, err := rule.NewTemplateFileRotateRule(
    "app-{{.CreateTime.Format \"2006-01-02\"}}.log",
    rule.WithMaxDuration(24 * time.Hour),
)
rw, _ := rotate_writer.NewRotateWriter(r, err)
```

### Custom rotation logic

```go
r, err := rule.NewFileRotateRule(
    func(count int, t time.Time, _ metered_writer.WriterState) string {
        return fmt.Sprintf("app-%d-%s.log", count, t.Format("150405"))
    },
    rule.WithLambda(func(state metered_writer.WriterState) bool {
        return state.Size > 5*1024*1024 || time.Since(state.CreatedAt) > time.Hour
    }),
    rule.WithRotateListener(func(count int, t time.Time, prev metered_writer.WriterState) {
        fmt.Printf("rotated to file #%d (prev size: %d bytes)\n", count, prev.Size)
    }),
)
```

### Available options

| Option | Description |
|---|---|
| `WithMaxSize(bytes int)` | Rotate when the current writer reaches this size |
| `WithMaxDuration(d time.Duration)` | Rotate when the writer has been open longer than `d` |
| `WithLambda(fn)` | Custom rotation predicate — full control |
| `WithRotateListener(fn)` | Callback fired after each rotation |
| `WithFileMode(flag, perm)` | Control `os.OpenFile` flags and permissions |


---

## License

MIT © Amin Tehrani
