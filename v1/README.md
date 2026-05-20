# rotate_writer (v1)
## DEPRECATED: Use github.com/amin-tehrani/rotate_writer/v2 instead


### Installation

```bash
go get github.com/amin-tehrani/rotate_writer/v1
```

### Usage

Define a `RotatorFn` that returns a new `io.WriteCloser` when rotation is needed, or `nil` to keep writing to the current one:

```go
import rotate_writer "github.com/amin-tehrani/rotate_writer/v1"

const maxSize = 10 * 1024 * 1024 // 10 MB

func rotateOnSize(status rotate_writer.RotateStatus) io.WriteCloser {
    if status.CurrentSize+status.AddedSize > maxSize {
        return openNewLogFile()
    }
    return nil
}

rw := rotate_writer.NewRotateWriter(openNewLogFile(), rotateOnSize)
defer rw.Close()

_, err := rw.Write([]byte("log line\n"))
```

---

## License

MIT © Amin Tehrani
