package preparation

import (
	"io"

	"github.com/fatih/color"
)

// logToStream writes colored messages to the output stream
func logToStream(stream io.Writer, message string, colorAttr color.Attribute) {
	if stream != nil {
		c := color.New(colorAttr)
		c.Fprintln(stream, message)
	}
}

// bufferedStreamWriter provides buffered streaming with periodic flushes
type bufferedStreamWriter struct {
	writer io.Writer
	buffer []byte
}

func newBufferedStreamWriter(writer io.Writer) *bufferedStreamWriter {
	return &bufferedStreamWriter{writer: writer}
}

func (b *bufferedStreamWriter) Write(p []byte) (n int, err error) {
	b.buffer = append(b.buffer, p...)
	if len(b.buffer) > 4096 { // Flush every 4KB
		b.Flush()
	}
	return len(p), nil
}

func (b *bufferedStreamWriter) Flush() error {
	if len(b.buffer) > 0 {
		_, err := b.writer.Write(b.buffer)
		b.buffer = b.buffer[:0]
		return err
	}
	return nil
}
