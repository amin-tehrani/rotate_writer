package rule

import (
	"os"
	"time"

	"github.com/amin-tehrani/rotate_writer/v2/metered_writer"
)

type FileRotateRule struct {
	generator      fileGenerator
	maxDuration    *time.Duration
	maxSize        int
	lambda         func(state metered_writer.WriterState) bool
	rotateListener func(count int, createTime time.Time, prevState metered_writer.WriterState)
}

type FileRotateOpt func(*FileRotateRule)

func (sfr *FileRotateRule) MaxSize() int {
	return sfr.maxSize
}

func (sfr *FileRotateRule) NewWriter(count int, createTime time.Time, prevState metered_writer.WriterState) (metered_writer.MeteredWriterCloser, error) {
	if sfr.rotateListener != nil {
		sfr.rotateListener(count, createTime, prevState)
	}
	return sfr.generator.NewWriter(count, createTime, prevState)
}
func (sfr *FileRotateRule) Check(state metered_writer.WriterState) bool {
	if sfr.maxSize > 0 && state.Size >= int64(sfr.maxSize) {
		return true
	}
	if sfr.maxDuration != nil && state.ModifiedAt.Sub(state.CreatedAt) > *sfr.maxDuration {
		return true
	}
	if sfr.lambda != nil && sfr.lambda(state) {
		return true
	}
	return false
}

func WithMaxDuration(d time.Duration) FileRotateOpt {
	return func(s *FileRotateRule) {
		s.maxDuration = &d
	}
}

func WithLambda(lambda func(state metered_writer.WriterState) bool) FileRotateOpt {
	return func(sfr *FileRotateRule) {
		sfr.lambda = lambda
	}
}

func WithRotateListener(l func(count int, createTime time.Time, prevState metered_writer.WriterState)) FileRotateOpt {
	return func(sfr *FileRotateRule) {
		sfr.rotateListener = l
	}
}

func WithMaxSize(maxSize int) FileRotateOpt {
	return func(sfr *FileRotateRule) {
		sfr.maxSize = maxSize
	}
}

func WithFileMode(flag int, perm os.FileMode) FileRotateOpt {
	return func(sfr *FileRotateRule) {
		sfr.generator.flag = flag
		sfr.generator.perm = perm
	}
}

type NameFn func(count int, createTime time.Time, prevState metered_writer.WriterState) string

func NewTemplateFileRotateRule(nameTemplate string, opts ...FileRotateOpt) (*FileRotateRule, error) {
	sfr := FileRotateRule{}
	fwt, err := newFileGeneratorTemplate(nameTemplate)
	if err != nil {
		return &sfr, err
	}
	sfr.generator = fwt
	for _, opt := range opts {
		opt(&sfr)
	}

	return &sfr, nil
}

func NewFileRotateRule(nameFn NameFn, opts ...FileRotateOpt) (*FileRotateRule, error) {
	sfr := FileRotateRule{}
	fwt, err := newFileGenerator(nameFn)
	if err != nil {
		return &sfr, err
	}
	sfr.generator = fwt
	for _, opt := range opts {
		opt(&sfr)
	}

	return &sfr, nil
}
