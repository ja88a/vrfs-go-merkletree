package service

import (
	"errors"
	"os"
	"path/filepath"
)

// Permissions of a stored file, which os.CreateTemp would otherwise restrict to its owner
const storedFilePerm os.FileMode = 0o644

// File is an upload in progress. Its data goes to a temporary file
// and only reaches its final path once committed.
type File struct {
	FilePath   string
	OutputFile *os.File
}

func NewFile() *File {
	return &File{}
}

// SetFile opens a temporary file in tmpDir for an upload targeting fileName in the directory path
func (f *File) SetFile(fileName, path, tmpDir string) error {
	if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil {
		return err
	}
	file, err := os.CreateTemp(tmpDir, "upload-*")
	if err != nil {
		return err
	}
	f.FilePath = filepath.Join(path, fileName)
	f.OutputFile = file
	return nil
}

func (f *File) Write(chunk []byte) error {
	if f.OutputFile == nil {
		return errors.New("no output file to write to")
	}
	_, err := f.OutputFile.Write(chunk)
	return err
}

// Commit flushes the temporary file to disk and atomically moves it to its final path
func (f *File) Commit() error {
	err := errors.Join(f.OutputFile.Chmod(storedFilePerm), f.OutputFile.Sync(), f.OutputFile.Close())
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f.FilePath), os.ModePerm); err != nil {
		return err
	}
	if err := os.Rename(f.OutputFile.Name(), f.FilePath); err != nil {
		return err
	}
	f.OutputFile = nil
	return nil
}

// Discard removes the temporary file of an upload that was not committed
func (f *File) Discard() error {
	if f.OutputFile == nil {
		return nil
	}
	name := f.OutputFile.Name()
	// The file is already closed if Commit failed after closing it
	if err := f.OutputFile.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		return errors.Join(err, os.Remove(name))
	}
	f.OutputFile = nil
	return os.Remove(name)
}
