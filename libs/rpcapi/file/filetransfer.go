package file

import (
	"io"
	"net/url"
	"strconv"
	"strings"

	"google.golang.org/grpc/metadata"
)

// HeaderFileName is a constant for the corresponding gRPC message field name
const HeaderFileName string = "file-name"

// HeaderFileType is a constant for the corresponding gRPC message field name
const HeaderFileType string = "file-type"

// HeaderFileSize is a constant for the corresponding gRPC message field name
const HeaderFileSize string = "file-size"

type File struct {
	r         io.Reader
	Name      string
	Extension string
	Size      int
}

// Metadata returns the gRPC headers describing the file.
// The name and extension are percent-encoded since gRPC headers only accept printable ASCII.
func (f *File) Metadata() metadata.MD {
	return metadata.New(map[string]string{
		HeaderFileName: url.PathEscape(f.Name),
		HeaderFileType: url.PathEscape(f.Extension),
		HeaderFileSize: strconv.Itoa(f.Size),
	})
}

func NewFromMetadata(md metadata.MD, r io.Reader) *File {
	name := decodeHeader(md, HeaderFileName)
	fileType := decodeHeader(md, HeaderFileType)
	var size int

	if sizes := md.Get(HeaderFileSize); len(sizes) > 0 {
		size, _ = strconv.Atoi(sizes[0])
	}

	return &File{Name: name, Extension: fileType, Size: size, r: r}
}

// decodeHeader returns the decoded value of a percent-encoded header, or an empty string if it is missing or malformed
func decodeHeader(md metadata.MD, key string) string {
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	value, err := url.PathUnescape(values[0])
	if err != nil {
		return ""
	}
	return value
}

func NewFile(name string, extension string, size int, r io.Reader) *File {
	return &File{Name: name, Extension: extension, Size: size, r: r}
}

func (f *File) Read(p []byte) (n int, err error) {
	return f.r.Read(p)
}

// Close releases the underlying reader when it supports it, which stops any transfer still feeding it
func (f *File) Close() error {
	if c, ok := f.r.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// IsSafeName reports whether name is a single path segment that cannot leave the directory it is joined to
func IsSafeName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\\\x00")
}
