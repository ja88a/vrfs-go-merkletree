package file

import (
	"io"
	"strconv"

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

func (f *File) Metadata() metadata.MD {
	return metadata.New(map[string]string{
		HeaderFileName: f.Name,
		HeaderFileType: f.Extension,
		HeaderFileSize: strconv.Itoa(f.Size),
	})
}

func NewFromMetadata(md metadata.MD, r io.Reader) *File {
	var name, fileType string
	var size int

	if names := md.Get(HeaderFileName); len(names) > 0 {
		name = names[0]
	}
	if types := md.Get(HeaderFileType); len(types) > 0 {
		fileType = types[0]
	}
	if sizes := md.Get(HeaderFileSize); len(sizes) > 0 {
		size, _ = strconv.Atoi(sizes[0])
	}

	return &File{Name: name, Extension: fileType, Size: size, r: r}
}

func NewFile(name string, extension string, size int, r io.Reader) *File {
	return &File{Name: name, Extension: extension, Size: size, r: r}
}

func (f *File) Read(p []byte) (n int, err error) {
	return f.r.Read(p)
}
