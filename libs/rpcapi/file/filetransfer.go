package file

import (
   "io"
   "strconv"

   "google.golang.org/grpc/metadata"
)

const HEARDER_FILENAME string = "file-name"
const HEADER_FILETYPE string  = "file-type"
const HEADER_FILESIZE string  = "file-size"

type File struct {
	r         io.Reader
	Name      string
	Extension string
	Size      int
}

func (f *File) Metadata() metadata.MD {
   return metadata.New(map[string]string{
      HEARDER_FILENAME: f.Name,
      HEADER_FILETYPE: f.Extension,
      HEADER_FILESIZE: strconv.Itoa(f.Size),
   })
}

func NewFromMetadata(md metadata.MD, r io.Reader) *File {
   var name, fileType string
   var size int

   if names := md.Get(HEARDER_FILENAME); len(names) > 0 {
      name = names[0]
   }
   if types := md.Get(HEADER_FILETYPE); len(types) > 0 {
      fileType = types[0]
   }
   if sizes := md.Get(HEADER_FILESIZE); len(sizes) > 0 {
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