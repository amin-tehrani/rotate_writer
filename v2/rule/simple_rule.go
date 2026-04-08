package rule

import (
	"os"
	"time"

	"github.com/amin-tehrani/rotate_writer/v2/metered_writer"
)

type SimpleFileRotateRule struct {
	generator   fileGenerator
	maxDuration *time.Duration
	maxSize     int
	lambda      func(state metered_writer.WriterState) bool
}

type SimpleOpt func(*SimpleFileRotateRule)

func (sfr *SimpleFileRotateRule) MaxSize() int {
	return sfr.maxSize
}

func (sfr *SimpleFileRotateRule) NewWriter(count int, createTime time.Time, prevState metered_writer.WriterState) (metered_writer.MeteredWriterCloser, error) {
	return sfr.generator.NewWriter(count, createTime, prevState)
}
func (sfr *SimpleFileRotateRule) Check(state metered_writer.WriterState) bool {
	if sfr.maxSize > 0 && state.Size < int64(sfr.maxSize) {
		return true
	}
	if sfr.maxDuration != nil && state.Modified.Sub(state.Created) > *sfr.maxDuration {
		return true
	}
	if sfr.lambda != nil && sfr.lambda(state) {
		return true
	}
	return false
}

func WithMaxDuration(d time.Duration) SimpleOpt {
	return func(s *SimpleFileRotateRule) {
		s.maxDuration = &d
	}
}

func WithLambda(lambda func(state metered_writer.WriterState) bool) SimpleOpt {
	return func(sfr *SimpleFileRotateRule) {
		sfr.lambda = lambda
	}
}

func WithMaxSize(maxSize int) SimpleOpt {
	return func(sfr *SimpleFileRotateRule) {
		sfr.maxSize = maxSize
	}
}

func WithFileMode(flag int, perm os.FileMode) SimpleOpt {
	return func(sfr *SimpleFileRotateRule) {
		sfr.generator.flag = flag
		sfr.generator.perm = perm
	}
}

type NameFn func(count int, createTime time.Time, prevState metered_writer.WriterState) string

func NewTemplateFileRotateRule(nameTemplate string, opts ...SimpleOpt) (*SimpleFileRotateRule, error) {
	sfr := SimpleFileRotateRule{}
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

func NewFileRotateRule(nameFn NameFn, opts ...SimpleOpt) (*SimpleFileRotateRule, error) {
	sfr := SimpleFileRotateRule{}
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
