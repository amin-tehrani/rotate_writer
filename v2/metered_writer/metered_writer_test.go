package metered_writer_test

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"

	"github.com/amin-tehrani/rotate_writer/v2/metered_writer"
)

// ---- NopWriterCloser ----

func TestNopWriterCloser_WriteDelegates(t *testing.T) {
	var buf bytes.Buffer
	w := metered_writer.NopWriterCloser(&buf)
	n, err := w.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if n != 5 {
		t.Fatalf("expected 5 bytes written, got %d", n)
	}
	if buf.String() != "hello" {
		t.Fatalf("expected buffer to contain 'hello', got %q", buf.String())
	}
}

func TestNopWriterCloser_CloseIsNop(t *testing.T) {
	var buf bytes.Buffer
	w := metered_writer.NopWriterCloser(&buf)
	if err := w.Close(); err != nil {
		t.Fatalf("expected Close to be a no-op, got error: %v", err)
	}
	// Write should still work after Close because it's a no-op
	_, err := w.Write([]byte("after close"))
	if err != nil {
		t.Fatalf("write after nop-close should succeed: %v", err)
	}
}

// ---- NewMeteredWriter (non-nil) ----

func TestNewMeteredWriter_Write_TracksSize(t *testing.T) {
	var buf bytes.Buffer
	mw := metered_writer.NewMeteredWriter(metered_writer.NopWriterCloser(&buf))

	if mw.State().Size != 0 {
		t.Fatalf("initial size should be 0, got %d", mw.State().Size)
	}

	_, err := mw.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mw.State().Size != 5 {
		t.Fatalf("expected size 5, got %d", mw.State().Size)
	}

	_, err = mw.Write([]byte("world"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mw.State().Size != 10 {
		t.Fatalf("expected size 10, got %d", mw.State().Size)
	}
}

func TestNewMeteredWriter_Write_UpdatesModifiedAt(t *testing.T) {
	var buf bytes.Buffer
	mw := metered_writer.NewMeteredWriter(metered_writer.NopWriterCloser(&buf))
	before := time.Now()
	_, _ = mw.Write([]byte("x"))
	after := time.Now()

	modAt := mw.State().ModifiedAt
	if modAt.Before(before) || modAt.After(after) {
		t.Fatalf("ModifiedAt %v not in expected range [%v, %v]", modAt, before, after)
	}
}

func TestNewMeteredWriter_State_CreatedAtSet(t *testing.T) {
	before := time.Now()
	var buf bytes.Buffer
	mw := metered_writer.NewMeteredWriter(metered_writer.NopWriterCloser(&buf))
	after := time.Now()

	created := mw.State().CreatedAt
	if created.Before(before) || created.After(after) {
		t.Fatalf("CreatedAt %v not in expected range [%v, %v]", created, before, after)
	}
}

func TestNewMeteredWriter_State_ClosedAtNilInitially(t *testing.T) {
	var buf bytes.Buffer
	mw := metered_writer.NewMeteredWriter(metered_writer.NopWriterCloser(&buf))
	if mw.State().ClosedAt != nil {
		t.Fatal("ClosedAt should be nil before Close is called")
	}
}

func TestNewMeteredWriter_Close_SetsClosedAt(t *testing.T) {
	var buf bytes.Buffer
	mw := metered_writer.NewMeteredWriter(metered_writer.NopWriterCloser(&buf))
	before := time.Now()
	if err := mw.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
	after := time.Now()

	closedAt := mw.State().ClosedAt
	if closedAt == nil {
		t.Fatal("ClosedAt should be set after Close")
	}
	if closedAt.Before(before) || closedAt.After(after) {
		t.Fatalf("ClosedAt %v not in expected range [%v, %v]", closedAt, before, after)
	}
}

func TestNewMeteredWriter_Close_DoubleCloseReturnsError(t *testing.T) {
	var buf bytes.Buffer
	mw := metered_writer.NewMeteredWriter(metered_writer.NopWriterCloser(&buf))
	if err := mw.Close(); err != nil {
		t.Fatalf("first close unexpected error: %v", err)
	}
	if err := mw.Close(); err != io.ErrClosedPipe {
		t.Fatalf("expected io.ErrClosedPipe on double close, got %v", err)
	}
}

func TestNewMeteredWriter_Write_PropagatesError(t *testing.T) {
	_, pw := io.Pipe()
	pw.Close() // close the write end; subsequent writes return io.ErrClosedPipe

	mw := metered_writer.NewMeteredWriter(pw)
	_, err := mw.Write([]byte("data"))
	if err == nil {
		t.Fatal("expected write error to propagate, got nil")
	}
	// Size should not increase on error
	if mw.State().Size != 0 {
		t.Fatalf("size should remain 0 after failed write, got %d", mw.State().Size)
	}
}

// ---- NewMeteredWriter(nil) ----

// Semantic bug: NewMeteredWriter(nil) uses os.Open(os.DevNull) which opens /dev/null
// with O_RDONLY. On Linux, the null device driver accepts writes regardless of open mode,
// so this does not fail at runtime — but it is semantically wrong.
// Fix: use os.OpenFile(os.DevNull, os.O_WRONLY, 0).
func TestNewMeteredWriter_Nil_WriteSucceeds(t *testing.T) {
	mw := metered_writer.NewMeteredWriter(nil)
	n, err := mw.Write([]byte("discard me"))
	if err != nil {
		t.Fatalf("write to NewMeteredWriter(nil) failed: %v", err)
	}
	if n != 10 {
		t.Fatalf("expected 10 bytes written, got %d", n)
	}
}

// ---- meteredFileWriter ----

func TestNewMeteredFileWriter_File(t *testing.T) {
	dir := t.TempDir()
	f, err := os.CreateTemp(dir, "test-*.log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer f.Close()

	mfw := metered_writer.NewMeteredFileWriter(f)
	if mfw.File() != f {
		t.Fatal("File() should return the same *os.File passed to NewMeteredFileWriter")
	}
}

func TestNewMeteredFileWriter_Write(t *testing.T) {
	dir := t.TempDir()
	f, err := os.CreateTemp(dir, "test-*.log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer f.Close()

	mfw := metered_writer.NewMeteredFileWriter(f)
	_, err = mfw.Write([]byte("hello file"))
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if mfw.State().Size != 10 {
		t.Fatalf("expected size 10, got %d", mfw.State().Size)
	}
}

func TestNewMeteredFileWriter_Close(t *testing.T) {
	dir := t.TempDir()
	f, err := os.CreateTemp(dir, "test-*.log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	mfw := metered_writer.NewMeteredFileWriter(f)
	if err := mfw.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
	if mfw.State().ClosedAt == nil {
		t.Fatal("ClosedAt should be set after Close")
	}
}

// ---- WriterState ----

func TestWriterState_String(t *testing.T) {
	ws := metered_writer.WriterState{}
	s := ws.String()
	if s == "" {
		t.Fatal("String() should return a non-empty string")
	}
}