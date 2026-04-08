package rule

import (
	"bytes"
	"errors"
	"html/template"
	"os"
	"time"

	"github.com/amin-tehrani/rotate_writer/v2/metered_writer"
)

type fileGenerator struct {
	nameFn NameFn
	flag   int
	perm   os.FileMode
}

func (fwt *fileGenerator) NewWriter(count int, createTime time.Time, prevState metered_writer.WriterState) (metered_writer.MeteredWriterCloser, error) {
	fileName := fwt.nameFn(count, createTime, prevState)

	if fileName == "" {
		return nil, errors.New("invalid fileName for new Writer: " + fileName)
	}
	flag := fwt.flag
	if flag == 0 {
		flag = os.O_CREATE | os.O_WRONLY
	}
	perm := fwt.perm
	if perm == 0 {
		perm = 0644
	}
	f, err := os.OpenFile(fileName, flag, perm)
	if err != nil {
		return nil, err
	}

	return metered_writer.NewMeteredFileWriter(f), nil
}

func newFileGenerator(nameFn NameFn) (fileGenerator, error) {
	return fileGenerator{
		nameFn: nameFn,
	}, nil
}

func newFileGeneratorTemplate(tmplStr string) (fileGenerator, error) {
	tmpl, err := template.New("fileGeneratorTemplate").Parse(tmplStr)
	if err != nil {
		return fileGenerator{}, err
	}
	nameFn := func(count int, createTime time.Time, prevState metered_writer.WriterState) string {
		var fileNameBuf bytes.Buffer
		if err := tmpl.Execute(&fileNameBuf, map[string]any{
			"count":      count,
			"createTime": createTime,
			"prevState":  prevState,
		}); err != nil {
			return ""
		}
		return fileNameBuf.String()
	}
	return newFileGenerator(nameFn)
}
