# rotate_writer
Generic GoLang Rotate Writer (io.Writer and io.WriterCloser) 


`rotate_writer` is a Go package that provides a `RotateWriter` struct, which implements an `io.Writer` that automatically rotates its underlying `io.WriteCloser` based on custom-defined conditions. This is useful for scenarios such as log file rotation where a new writer needs to be created periodically or based on certain size or time conditions.

## Installation

To use `rotate_writer`, install it using:

```shell
go get -u github.com/amin-tehrani/rotate_writer
```


## Usage
Creating a RotateWriter
First, you need to define a rotation condition function of type `RotatorFn`, which takes a `RotateStatus` and returns a new `io.WriteCloser` when a rotation is needed. Here’s an example condition that rotates based on a maximum size:

The name, type, and rotation triggerer can be fully customized within this function.

```go
func rotateOnSize(status rotate_writer.RotateStatus) io.WriteCloser {
    if status.CurrentSize+status.AddedSize > maxSize {
        // Return a new writer if the size exceeds maxSize
        return openNewLogFile() // Can be buffer, stdio, logger or any other kind of Writers 
    }
    return nil
}

```

Then, create a `RotateWriter` with an initial writer and the condition function:
```go
initialWriter := openNewLogFile() // Initial writer (e.g., an open file)
rw := rotate_writer.NewRotateWriter(initialWriter, rotateOnSize)
```

**Writing to RotateWriter**
To write data, simply call the `Write` method:

```go
_, err := rw.Write([]byte("Some log message")) // implemented io.Writer
if err != nil {
    log.Fatalf("Failed to write: %v", err)
}
```

The `RotateWriter` will automatically rotate the writer if the rotateCondition returns a new writer.

## Manually Rotating the Writer
If needed, you can manually rotate the writer by calling Rotate with a new io.WriteCloser


**Closing the RotateWriter**
Always close the RotateWriter to ensure all resources are properly released:

```go
err := rw.Close()  // implemented io.WriteCloser
if err != nil {
    log.Fatalf("Failed to close: %v", err)
}
```
