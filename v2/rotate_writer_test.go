package rotate_writer_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	rotate_writer "github.com/amin-tehrani/rotate_writer/v2"
	"github.com/amin-tehrani/rotate_writer/v2/metered_writer"
	"github.com/amin-tehrani/rotate_writer/v2/rule"
)

// ---- helpers ----

func newFileRule(t *testing.T, dir string, opts ...rule.FileRotateOpt) *rule.FileRotateRule {
	t.Helper()
	r, err := rule.NewFileRotateRule(
		func(count int, _ time.Time, _ metered_writer.WriterState) string {
			return filepath.Join(dir, fmt.Sprintf("log-%d.log", count))
		},
		opts...,
	)
	if err != nil {
		t.Fatalf("newFileRule: %v", err)
	}
	return r
}

func newWriter(t *testing.T, r *rule.FileRotateRule) *rotate_writer.RotateWriter {
	t.Helper()
	rw, err := rotate_writer.NewRotateWriter(r, nil)
	if err != nil {
		t.Fatalf("NewRotateWriter: %v", err)
	}
	return rw
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readFile %q: %v", path, err)
	}
	return data
}

// ---- NewRotateWriter ----

func TestNewRotateWriter_CreatesFirstFile(t *testing.T) {
	dir := t.TempDir()
	r := newFileRule(t, dir)
	rw := newWriter(t, r)
	defer rw.Close()

	if _, err := os.Stat(filepath.Join(dir, "log-1.log")); os.IsNotExist(err) {
		t.Fatal("first file log-1.log should be created on construction")
	}
}

func TestNewRotateWriter_PropagatesRuleError(t *testing.T) {
	_, err := rotate_writer.NewRotateWriter(nil, fmt.Errorf("rule init error"))
	if err == nil {
		t.Fatal("expected error to be propagated")
	}
}

// ---- Write ----

func TestRotateWriter_Write_BasicWrite(t *testing.T) {
	dir := t.TempDir()
	rw := newWriter(t, newFileRule(t, dir))
	defer rw.Close()

	data := []byte("hello rotate\n")
	n, err := rw.Write(data)
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if n != len(data) {
		t.Fatalf("expected %d bytes written, got %d", len(data), n)
	}

	got := readFile(t, filepath.Join(dir, "log-1.log"))
	if string(got) != string(data) {
		t.Fatalf("file content mismatch: got %q, want %q", got, data)
	}
}

func TestRotateWriter_Write_RotatesOnLambda(t *testing.T) {
	dir := t.TempDir()
	// Rotate once the file reaches 6 bytes ("first\n"). After rotation the new
	// writer starts at size 0, so the lambda returns false and recursion stops.
	r := newFileRule(t, dir, rule.WithLambda(func(state metered_writer.WriterState) bool {
		return state.Size >= 6
	}))
	rw := newWriter(t, r)
	defer rw.Close()

	rw.Write([]byte("first\n"))  // fills to 6 bytes; next Check triggers rotation
	rw.Write([]byte("second\n")) // triggers rotation, then lands on log-2.log
	rw.Write([]byte("third\n"))  // still on log-2.log (not yet at threshold)

	// "second\n" triggers rotation (size was 6 ≥ threshold) then lands in log-2.log.
	// After writing "second\n" size is 7 ≥ 6, so "third\n" triggers another rotation
	// and lands in log-3.log.
	if _, err := os.Stat(filepath.Join(dir, "log-2.log")); os.IsNotExist(err) {
		t.Fatal("expected log-2.log to be created after first rotation")
	}
	got2 := readFile(t, filepath.Join(dir, "log-2.log"))
	if string(got2) != "second\n" {
		t.Fatalf("log-2.log: expected 'second\\n', got %q", got2)
	}
	if _, err := os.Stat(filepath.Join(dir, "log-3.log")); os.IsNotExist(err) {
		t.Fatal("expected log-3.log to be created after second rotation")
	}
	got3 := readFile(t, filepath.Join(dir, "log-3.log"))
	if string(got3) != "third\n" {
		t.Fatalf("log-3.log: expected 'third\\n', got %q", got3)
	}
}

func TestRotateWriter_Write_SplitWriteOnMaxSize(t *testing.T) {
	dir := t.TempDir()
	const maxSize = 10
	rw := newWriter(t, newFileRule(t, dir, rule.WithMaxSize(maxSize)))
	defer rw.Close()

	// Write exactly maxSize bytes — fills the first file
	rw.Write([]byte("1234567890"))
	// Write more — should split: rotate and write remainder to new file
	rw.Write([]byte("abcde"))

	file1 := readFile(t, filepath.Join(dir, "log-1.log"))
	if string(file1) != "1234567890" {
		t.Fatalf("log-1.log: expected '1234567890', got %q", file1)
	}
	file2 := readFile(t, filepath.Join(dir, "log-2.log"))
	if string(file2) != "abcde" {
		t.Fatalf("log-2.log: expected 'abcde', got %q", file2)
	}
}

func TestRotateWriter_Write_SplitWriteCrossingBoundary(t *testing.T) {
	dir := t.TempDir()
	const maxSize = 10
	rw := newWriter(t, newFileRule(t, dir, rule.WithMaxSize(maxSize)))
	defer rw.Close()

	// Write 7 bytes first, then a 10-byte payload that crosses the boundary at byte 3
	rw.Write([]byte("abcdefg"))
	rw.Write([]byte("1234567890")) // 3 bytes go to file 1, 7 to file 2

	file1 := readFile(t, filepath.Join(dir, "log-1.log"))
	if string(file1) != "abcdefg123" {
		t.Fatalf("log-1.log: expected 'abcdefg123', got %q", file1)
	}
	file2 := readFile(t, filepath.Join(dir, "log-2.log"))
	if string(file2) != "4567890" {
		t.Fatalf("log-2.log: expected '4567890', got %q", file2)
	}
}

func TestRotateWriter_Write_MaxSize_ExactFit_NoSpuriousRotation(t *testing.T) {
	dir := t.TempDir()
	const maxSize = 5
	rw := newWriter(t, newFileRule(t, dir, rule.WithMaxSize(maxSize)))
	defer rw.Close()

	rw.Write([]byte("abc"))
	rw.Write([]byte("de")) // fills to exactly maxSize; Check returns true, rotation on NEXT write

	// At this point we're at maxSize exactly; the next write should trigger rotation
	rw.Write([]byte("X"))

	if _, err := os.Stat(filepath.Join(dir, "log-2.log")); os.IsNotExist(err) {
		t.Fatal("expected log-2.log after hitting maxSize")
	}
}

// ---- Rotate ----

func TestRotateWriter_Rotate_ClosesCurrentFile(t *testing.T) {
	dir := t.TempDir()
	rw := newWriter(t, newFileRule(t, dir))
	defer rw.Close()

	stateBefore := rw.State()
	if stateBefore.ClosedAt != nil {
		t.Fatal("ClosedAt should be nil before Rotate")
	}

	if err := rw.Rotate(); err != nil {
		t.Fatalf("Rotate error: %v", err)
	}

	// After Rotate, the OLD writer's state should have ClosedAt set.
	// The new embedded MeteredWriterCloser is fresh.
	if rw.State().ClosedAt != nil {
		t.Fatal("new writer's ClosedAt should be nil after Rotate")
	}
	if _, err := os.Stat(filepath.Join(dir, "log-2.log")); os.IsNotExist(err) {
		t.Fatal("expected log-2.log after explicit Rotate")
	}
}

func TestRotateWriter_MultipleRotations(t *testing.T) {
	dir := t.TempDir()
	rw := newWriter(t, newFileRule(t, dir))
	defer rw.Close()

	for i := 0; i < 4; i++ {
		if err := rw.Rotate(); err != nil {
			t.Fatalf("Rotate %d: %v", i, err)
		}
	}

	// After 4 explicit rotations (plus the initial one in NewRotateWriter), we should have log-5.log
	if _, err := os.Stat(filepath.Join(dir, "log-5.log")); os.IsNotExist(err) {
		t.Fatal("expected log-5.log after 4 rotations")
	}
}

// ---- File() ----

func TestRotateWriter_File_ReturnsFile(t *testing.T) {
	dir := t.TempDir()
	rw := newWriter(t, newFileRule(t, dir))
	defer rw.Close()

	f := rw.File()
	if f == nil {
		t.Fatal("File() should return a non-nil *os.File for a file-backed rule")
	}
}

// ---- Close ----

func TestRotateWriter_Close_ClosesUnderlyingFile(t *testing.T) {
	dir := t.TempDir()
	rw := newWriter(t, newFileRule(t, dir))

	if err := rw.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
	if rw.State().ClosedAt == nil {
		t.Fatal("ClosedAt should be set after Close")
	}
}

func TestRotateWriter_Close_DoubleClose(t *testing.T) {
	dir := t.TempDir()
	rw := newWriter(t, newFileRule(t, dir))

	if err := rw.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	// Second close should return an error (io.ErrClosedPipe from baseMeteredWriter)
	if err := rw.Close(); err == nil {
		t.Fatal("expected error on double close")
	}
}

// ---- Concurrent writes ----

func TestRotateWriter_ConcurrentWrites_NoDataRace(t *testing.T) {
	dir := t.TempDir()
	rw := newWriter(t, newFileRule(t, dir, rule.WithMaxSize(50)))
	defer rw.Close()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				rw.Write([]byte(fmt.Sprintf("goroutine-%d-write-%d\n", i, j)))
			}
		}(i)
	}
	wg.Wait()
}

// TestRotateWriter_ConcurrentWrites_DataIntegrity verifies that under concurrent
// load with rotation, no bytes are silently dropped: the total bytes across all
// rotated files equals the total bytes reported as written by Write().
func TestRotateWriter_ConcurrentWrites_DataIntegrity(t *testing.T) {
	dir := t.TempDir()
	const maxSize = 64
	rw := newWriter(t, newFileRule(t, dir, rule.WithMaxSize(maxSize)))

	const goroutines = 20
	const writesEach = 50
	payload := []byte("hello!!\n") // 8 bytes

	var totalWritten atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < writesEach; j++ {
				n, err := rw.Write(payload)
				if err == nil {
					totalWritten.Add(int64(n))
				}
			}
		}()
	}
	wg.Wait()

	if err := rw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var totalInFiles int64
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("ReadFile %q: %v", e.Name(), err)
		}
		totalInFiles += int64(len(data))
	}

	if totalInFiles != totalWritten.Load() {
		t.Fatalf("data integrity: %d bytes written, but %d bytes found in files",
			totalWritten.Load(), totalInFiles)
	}
}

// TestRotateWriter_ConcurrentWrites_OldFilesClosedBeforeRotation verifies that
// every time a rotation happens, the previous file's ClosedAt is set — meaning
// the file was properly closed before the next one was opened.
func TestRotateWriter_ConcurrentWrites_OldFilesClosedBeforeRotation(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	var unclosedCount int

	r, err := rule.NewFileRotateRule(
		func(count int, _ time.Time, _ metered_writer.WriterState) string {
			return filepath.Join(dir, fmt.Sprintf("log-%d.log", count))
		},
		rule.WithMaxSize(32),
		rule.WithRotateListener(func(_ int, _ time.Time, prev metered_writer.WriterState) {
			if prev.ClosedAt == nil {
				mu.Lock()
				unclosedCount++
				mu.Unlock()
			}
		}),
	)
	if err != nil {
		t.Fatalf("NewFileRotateRule: %v", err)
	}
	rw, err := rotate_writer.NewRotateWriter(r, nil)
	if err != nil {
		t.Fatalf("NewRotateWriter: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				rw.Write([]byte("rotation test payload\n"))
			}
		}()
	}
	wg.Wait()
	rw.Close()

	mu.Lock()
	defer mu.Unlock()
	if unclosedCount > 0 {
		t.Fatalf("%d rotation(s) occurred where the previous file was not closed before rotation", unclosedCount)
	}
}

// TestRotateWriter_BrokenState_AfterRotationFailure verifies that after a failed
// rotation (rule.NewWriter errors), subsequent Write calls return an error rather
// than silently writing to the already-closed underlying file.
func TestRotateWriter_BrokenState_AfterRotationFailure(t *testing.T) {
	dir := t.TempDir()

	// This rule fails on the second NewWriter call. The first Write that triggers
	// the split-write rotation will get an error. After that, the RotateWriter
	// holds a closed file — writes must keep returning errors.
	rw, err := rotate_writer.NewRotateWriter(&failOnSecondRotateRule{dir: dir}, nil)
	if err != nil {
		t.Fatalf("NewRotateWriter: %v", err)
	}

	// Fill to 4 bytes (maxSize=5, so one more byte triggers split-write rotation)
	if _, err := rw.Write([]byte("abcd")); err != nil {
		t.Fatalf("first write: %v", err)
	}

	// This triggers split-write: writes 1 byte, rotation fails.
	_, firstErr := rw.Write([]byte("efgh"))
	if firstErr == nil {
		t.Fatal("expected rotation failure error")
	}

	// The writer is now in a broken state (MeteredWriterCloser is closed).
	// Every subsequent write must return an error — not silently succeed.
	_, secondErr := rw.Write([]byte("more data"))
	if secondErr == nil {
		t.Fatal("write after broken state should return an error, got nil")
	}
}

// TestRotateWriter_Close_FlushesDataBeforeClosing verifies that all data written
// before Close() is readable from disk after Close() returns.
func TestRotateWriter_Close_FlushesDataBeforeClosing(t *testing.T) {
	dir := t.TempDir()
	rw := newWriter(t, newFileRule(t, dir))

	data := []byte("important data that must be flushed\n")
	if _, err := rw.Write(data); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := rw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got := readFile(t, filepath.Join(dir, "log-1.log"))
	if string(got) != string(data) {
		t.Fatalf("after Close, file content = %q; want %q", got, data)
	}
}

// ---- Bug 1: infinite recursion / excessive rotation with stateless lambda ----

// BUG 1: When rule.Check() returns true, Write() rotates and then RECURSIVELY calls
// Write(p) again. If the lambda does not depend on writer state (e.g. a global counter,
// or always-true), this causes infinite recursion → stack overflow.
//
// The lambda here is capped at 50 true-returns to prevent a real stack overflow in the
// test binary. After the cap it returns false so the recursion eventually stops.
// A correct implementation should rotate ONCE and then write — not re-check after rotation.
// Fix: after rotation triggered by Check(), write directly instead of recursing.
func TestBug_StatelessLambda_ShouldNotCauseExcessiveRotations(t *testing.T) {
	dir := t.TempDir()
	rotations := 0
	callCount := 0

	r := newFileRule(t, dir,
		rule.WithLambda(func(_ metered_writer.WriterState) bool {
			callCount++
			return callCount <= 50 // cap to prevent real stack overflow
		}),
		rule.WithRotateListener(func(_ int, _ time.Time, _ metered_writer.WriterState) {
			rotations++
		}),
	)
	rw := newWriter(t, r)
	defer rw.Close()

	// NewRotateWriter calls Rotate() once during construction to open the first file.
	// Capture that baseline so we only measure rotations triggered by Write.
	rotationsAtStart := rotations

	// Write a single payload. Correct behaviour: rotate once, write to the new file.
	// Buggy behaviour: rotate up to 50 times before the cap kicks in.
	rw.Write([]byte("hello"))

	extraRotations := rotations - rotationsAtStart
	if extraRotations > 1 {
		t.Fatalf("BUG: a single Write triggered %d extra rotations (expected 1); "+
			"stateless lambda causes recursive re-check after each rotation", extraRotations)
	}

	// Data should land in the file opened after the one rotation triggered by Write.
	expectedFile := filepath.Join(dir, "log-2.log")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Fatalf("expected data file %q does not exist", expectedFile)
	}
	got := readFile(t, expectedFile)
	if string(got) != "hello" {
		t.Fatalf("expected 'hello' in log-2.log, got %q", got)
	}
}

// ---- Split-write error paths ----

// errWriter is a WriteCloser whose Write always fails.
type errWriter struct{}

func (*errWriter) Write(_ []byte) (int, error) { return 0, fmt.Errorf("write error") }
func (*errWriter) Close() error                { return nil }

// alwaysFailWriteRule returns a writer that always fails on Write.
// Implementing MaxSize makes it eligible for the split-write path.
type alwaysFailWriteRule struct{ maxSize int }

func (r *alwaysFailWriteRule) Check(_ metered_writer.WriterState) bool { return false }
func (r *alwaysFailWriteRule) MaxSize() int                            { return r.maxSize }
func (r *alwaysFailWriteRule) NewWriter(_ int, _ time.Time, _ metered_writer.WriterState) (metered_writer.MeteredWriterCloser, error) {
	return metered_writer.NewMeteredWriter(&errWriter{}), nil
}

func TestRotateWriter_Write_SplitWrite_WriteError(t *testing.T) {
	// maxSize=3, write 4 bytes → split triggered; Write(p1) fails immediately.
	rw, err := rotate_writer.NewRotateWriter(&alwaysFailWriteRule{maxSize: 3}, nil)
	if err != nil {
		t.Fatalf("NewRotateWriter: %v", err)
	}
	_, err = rw.Write([]byte("abcd"))
	if err == nil {
		t.Fatal("expected error when underlying Write fails during split-write")
	}
}

// failOnSecondRotateRule succeeds on the first NewWriter call (used by constructor)
// and fails on subsequent calls (triggered by split-write rotation).
type failOnSecondRotateRule struct {
	dir   string
	calls int
}

func (r *failOnSecondRotateRule) Check(_ metered_writer.WriterState) bool { return false }
func (r *failOnSecondRotateRule) MaxSize() int                            { return 5 }
func (r *failOnSecondRotateRule) NewWriter(count int, _ time.Time, _ metered_writer.WriterState) (metered_writer.MeteredWriterCloser, error) {
	r.calls++
	if r.calls > 1 {
		return nil, fmt.Errorf("intentional rotation failure")
	}
	f, err := os.CreateTemp(r.dir, fmt.Sprintf("log-%d-*.log", count))
	if err != nil {
		return nil, err
	}
	return metered_writer.NewMeteredFileWriter(f), nil
}

func TestRotateWriter_Write_SplitWrite_RotateError(t *testing.T) {
	dir := t.TempDir()
	rw, err := rotate_writer.NewRotateWriter(&failOnSecondRotateRule{dir: dir}, nil)
	if err != nil {
		t.Fatalf("NewRotateWriter: %v", err)
	}
	// Write 4 bytes to get currSize=4, then write 4 more.
	// Second write: int(4)+4 > 5 → split; p1="e" fills to 5, Rotate fails.
	if _, err := rw.Write([]byte("abcd")); err != nil {
		t.Fatalf("first write: %v", err)
	}
	_, err = rw.Write([]byte("efgh"))
	if err == nil {
		t.Fatal("expected error when Rotate fails during split-write")
	}
}

// ---- RotateError ----

func TestRotateError_Error(t *testing.T) {
	// Trigger a rotation error by providing a NameFn that returns a non-existent directory path
	r, err := rule.NewFileRotateRule(func(_ int, _ time.Time, _ metered_writer.WriterState) string {
		return "/nonexistent-dir-xyz/file.log"
	})
	if err != nil {
		t.Fatalf("unexpected rule error: %v", err)
	}
	_, err = rotate_writer.NewRotateWriter(r, nil)
	if err == nil {
		t.Fatal("expected error when file cannot be created")
	}
}

// ---- File() nil branch ----

// customRule implements rule.RotateRule but NOT rule.RotateSizer, so Write()
// will skip the split-on-boundary path. Its writers are non-file MeteredWriterClosers,
// so File() returns nil.
type inMemoryRule struct {
	buf *bytes.Buffer
}

func (r *inMemoryRule) Check(_ metered_writer.WriterState) bool { return false }
func (r *inMemoryRule) NewWriter(_ int, _ time.Time, _ metered_writer.WriterState) (metered_writer.MeteredWriterCloser, error) {
	r.buf = &bytes.Buffer{}
	return metered_writer.NewMeteredWriter(metered_writer.NopWriterCloser(r.buf)), nil
}

func TestRotateWriter_File_ReturnsNil_ForNonFileWriter(t *testing.T) {
	rw, err := rotate_writer.NewRotateWriter(&inMemoryRule{}, nil)
	if err != nil {
		t.Fatalf("NewRotateWriter: %v", err)
	}
	defer rw.Close()

	if rw.File() != nil {
		t.Fatal("File() should return nil for a non-file-backed writer")
	}
}

func TestRotateWriter_Write_NoRotateSizer_UsesCheckOnly(t *testing.T) {
	r := &inMemoryRule{}
	rw, err := rotate_writer.NewRotateWriter(r, nil)
	if err != nil {
		t.Fatalf("NewRotateWriter: %v", err)
	}
	defer rw.Close()

	data := []byte("hello in-memory")
	n, err := rw.Write(data)
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if n != len(data) {
		t.Fatalf("expected %d bytes, got %d", len(data), n)
	}
}