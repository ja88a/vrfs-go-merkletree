package file

import (
   "io"
   "strconv"

   "google.golang.org/grpc/metadata"
)

var fileNameHeader = "file-name"
var fileTypeHeader = "file-type"
var fileSizeHeader = "file-size"

type File struct {
	r         io.Reader
	Name      string
	Extension string
	Size      int
}

func (f *File) Metadata() metadata.MD {
   return metadata.New(map[string]string{
      fileNameHeader: f.Name,
      fileTypeHeader: f.Extension,
      fileSizeHeader: strconv.Itoa(f.Size),
   })
}

func NewFromMetadata(md metadata.MD, r io.Reader) *File {
   var name, fileType string
   var size int

   if names := md.Get(fileNameHeader); len(names) > 0 {
      name = names[0]
   }
   if types := md.Get(fileTypeHeader); len(types) > 0 {
      fileType = types[0]
   }
   if sizes := md.Get(fileSizeHeader); len(sizes) > 0 {
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