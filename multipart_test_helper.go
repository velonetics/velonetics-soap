package soap

import (
	"bytes"
	"mime/multipart"
)

type multipartWriterCompat struct {
	*multipart.Writer
}

func newMultipartWriter(buf *bytes.Buffer) *multipartWriterCompat {
	return &multipartWriterCompat{multipart.NewWriter(buf)}
}
