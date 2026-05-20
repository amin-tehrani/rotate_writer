package rule_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amin-tehrani/rotate_writer/v2/metered_writer"
	"github.com/amin-tehrani/rotate_writer/v2/rule"
)

// ---- helpers ----

func nameFn(dir string) rule.NameFn {
	return func(count int, _ time.Time, _ metered_writer.WriterState) string {
		return filepath.Join(dir, fmt.Sprintf("test-%d.log", count))
	}
}

func stateWithSize(size int64) metered_writer.WriterState {
	return metered_writer.WriterState{
		CreatedAt:  time.Now().Add(-time.Minute),
		ModifiedAt: time.Now(),
		Size:       size,
	}
}

// ---- FileRotateRule.Check ----

func TestFileRotateRule_Check_NoCondition(t *testing.T) {
	dir := t.TempDir()
	r, err := rule.NewFileRotateRule(nameFn(dir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Check(stateWithSize(1024 * 1024)) {
		t.Fatal("Check should return false when no conditions are set")
	}
}

func TestFileRotateRule_Check_MaxSize_NotReached(t *testing.T) {
	dir := t.TempDir()
	r, err := rule.NewFileRotateRule(nameFn(dir), rule.WithMaxSize(100))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Check(stateWithSize(99)) {
		t.Fatal("Check should return false when size < maxSize")
	}
}

func TestFileRotateRule_Check_MaxSize_Reached(t *testing.T) {
	dir := t.TempDir()
	r, err := rule.NewFileRotateRule(nameFn(dir), rule.WithMaxSize(100))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.Check(stateWithSize(100)) {
		t.Fatal("Check should return true when size >= maxSize")
	}
}

func TestFileRotateRule_Check_MaxSize_Exceeded(t *testing.T) {
	dir := t.TempDir()
	r, err := rule.NewFileRotateRule(nameFn(dir), rule.WithMaxSize(100))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.Check(stateWithSize(200)) {
		t.Fatal("Check should return true when size > maxSize")
	}
}

func TestFileRotateRule_Check_MaxDuration_NotReached(t *testing.T) {
	dir := t.TempDir()
	r, err := rule.NewFileRotateRule(nameFn(dir), rule.WithMaxDuration(time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	state := metered_writer.WriterState{
		CreatedAt:  time.Now(),
		ModifiedAt: time.Now(),
	}
	if r.Check(state) {
		t.Fatal("Check should return false when duration has not been exceeded")
	}
}

func TestFileRotateRule_Check_MaxDuration_Exceeded(t *testing.T) {
	dir := t.TempDir()
	r, err := rule.NewFileRotateRule(nameFn(dir), rule.WithMaxDuration(time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	state := metered_writer.WriterState{
		CreatedAt:  time.Now().Add(-2 * time.Minute),
		ModifiedAt: time.Now(), // elapsed = ModifiedAt - CreatedAt = 2min > 1min
	}
	if !r.Check(state) {
		t.Fatal("Check should return true when duration has been exceeded")
	}
}

func TestFileRotateRule_Check_Lambda_True(t *testing.T) {
	dir := t.TempDir()
	r, err := rule.NewFileRotateRule(nameFn(dir), rule.WithLambda(func(_ metered_writer.WriterState) bool {
		return true
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.Check(stateWithSize(0)) {
		t.Fatal("Check should return true when lambda returns true")
	}
}

func TestFileRotateRule_Check_Lambda_False(t *testing.T) {
	dir := t.TempDir()
	r, err := rule.NewFileRotateRule(nameFn(dir), rule.WithLambda(func(_ metered_writer.WriterState) bool {
		return false
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Check(stateWithSize(0)) {
		t.Fatal("Check should return false when lambda returns false")
	}
}

// ---- FileRotateRule.MaxSize ----

func TestFileRotateRule_MaxSize(t *testing.T) {
	dir := t.TempDir()
	r, err := rule.NewFileRotateRule(nameFn(dir), rule.WithMaxSize(1234))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.MaxSize() != 1234 {
		t.Fatalf("expected MaxSize 1234, got %d", r.MaxSize())
	}
}

func TestFileRotateRule_MaxSize_Zero_WhenNotSet(t *testing.T) {
	dir := t.TempDir()
	r, err := rule.NewFileRotateRule(nameFn(dir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.MaxSize() != 0 {
		t.Fatalf("expected MaxSize 0 when not set, got %d", r.MaxSize())
	}
}

// ---- FileRotateRule.NewWriter ----

func TestFileRotateRule_NewWriter_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	r, err := rule.NewFileRotateRule(nameFn(dir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	w, err := r.NewWriter(1, time.Now(), metered_writer.WriterState{})
	if err != nil {
		t.Fatalf("unexpected error from NewWriter: %v", err)
	}
	defer w.Close()

	expectedPath := filepath.Join(dir, "test-1.log")
	if _, statErr := os.Stat(expectedPath); os.IsNotExist(statErr) {
		t.Fatalf("expected file %q to be created", expectedPath)
	}
}

func TestFileRotateRule_NewWriter_CallsRotateListener(t *testing.T) {
	dir := t.TempDir()
	listenerCalled := false
	var gotCount int
	r, err := rule.NewFileRotateRule(
		nameFn(dir),
		rule.WithRotateListener(func(count int, _ time.Time, _ metered_writer.WriterState) {
			listenerCalled = true
			gotCount = count
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	w, err := r.NewWriter(3, time.Now(), metered_writer.WriterState{})
	if err != nil {
		t.Fatalf("unexpected error from NewWriter: %v", err)
	}
	defer w.Close()

	if !listenerCalled {
		t.Fatal("rotateListener should have been called")
	}
	if gotCount != 3 {
		t.Fatalf("expected count 3, got %d", gotCount)
	}
}

func TestFileRotateRule_NewWriter_EmptyFileName_ReturnsError(t *testing.T) {
	emptyNameFn := func(_ int, _ time.Time, _ metered_writer.WriterState) string {
		return ""
	}
	r, err := rule.NewFileRotateRule(emptyNameFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = r.NewWriter(1, time.Now(), metered_writer.WriterState{})
	if err == nil {
		t.Fatal("expected error when file name is empty")
	}
}

func TestFileRotateRule_NewWriter_WithFileMode(t *testing.T) {
	dir := t.TempDir()
	r, err := rule.NewFileRotateRule(
		nameFn(dir),
		rule.WithFileMode(os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	w, err := r.NewWriter(1, time.Now(), metered_writer.WriterState{})
	if err != nil {
		t.Fatalf("unexpected error from NewWriter: %v", err)
	}
	defer w.Close()

	info, err := os.Stat(filepath.Join(dir, "test-1.log"))
	if err != nil {
		t.Fatalf("stat error: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("expected perm 0600, got %v", info.Mode().Perm())
	}
}

// ---- NewTemplateFileRotateRule ----

func TestNewTemplateFileRotateRule_ValidTemplate(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, `app-{{printf "%03d" .Count}}.log`)
	r, err := rule.NewTemplateFileRotateRule(tmpl)
	if err != nil {
		t.Fatalf("unexpected error for valid template: %v", err)
	}
	if r == nil {
		t.Fatal("expected non-nil rule")
	}
	w, err := r.NewWriter(1, time.Now(), metered_writer.WriterState{})
	if err != nil {
		t.Fatalf("unexpected error from NewWriter: %v", err)
	}
	defer w.Close()

	expectedPath := filepath.Join(dir, "app-001.log")
	if _, statErr := os.Stat(expectedPath); os.IsNotExist(statErr) {
		t.Fatalf("expected file %q to be created from template", expectedPath)
	}
}

func TestNewTemplateFileRotateRule_CountKey(t *testing.T) {
	dir := t.TempDir()
	tmplStr := filepath.Join(dir, `app-{{printf "%03d" .Count}}.log`)
	r, err := rule.NewTemplateFileRotateRule(tmplStr)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	w, err := r.NewWriter(1, time.Now(), metered_writer.WriterState{})
	if err != nil {
		t.Fatalf("unexpected error from NewWriter: %v", err)
	}
	defer w.Close()

	expectedPath := filepath.Join(dir, "app-001.log")
	if _, statErr := os.Stat(expectedPath); os.IsNotExist(statErr) {
		t.Fatalf("expected file %q to be created using .Count", expectedPath)
	}
}

func TestNewTemplateFileRotateRule_CreateTimeKey(t *testing.T) {
	dir := t.TempDir()
	tmplStr := filepath.Join(dir, `app-{{.CreateTime.Format "2006-01-02"}}.log`)
	r, err := rule.NewTemplateFileRotateRule(tmplStr)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	today := time.Now().Format("2006-01-02")
	w, err := r.NewWriter(1, time.Now(), metered_writer.WriterState{})
	if err != nil {
		t.Fatalf("unexpected error from NewWriter: %v", err)
	}
	defer w.Close()

	expectedPath := filepath.Join(dir, "app-"+today+".log")
	if _, statErr := os.Stat(expectedPath); os.IsNotExist(statErr) {
		t.Fatalf("expected file %q to be created using .CreateTime", expectedPath)
	}
}

func TestNewTemplateFileRotateRule_InvalidTemplate(t *testing.T) {
	_, err := rule.NewTemplateFileRotateRule("{{.unclosed")
	if err == nil {
		t.Fatal("expected error for invalid template syntax")
	}
}

func TestNewTemplateFileRotateRule_EmptyTemplateName_ReturnsError(t *testing.T) {
	// A template that produces an empty string should fail on NewWriter
	r, err := rule.NewTemplateFileRotateRule("")
	if err != nil {
		t.Fatalf("unexpected parse error for empty template: %v", err)
	}
	_, err = r.NewWriter(1, time.Now(), metered_writer.WriterState{})
	if err == nil {
		t.Fatal("expected error when template produces empty filename")
	}
}