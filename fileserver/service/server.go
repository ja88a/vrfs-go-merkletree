package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/ja88a/vrfs-go-merkletree/libs/protos/v1/fileserver"
	config "github.com/ja88a/vrfs-go-merkletree/libs/utils/config"
	file "github.com/ja88a/vrfs-go-merkletree/libs/utils/file"
	futils "github.com/ja88a/vrfs-go-merkletree/libs/merkletree/files"
	logger "github.com/ja88a/vrfs-go-merkletree/libs/utils/logger"
)

// Execution context of the service
type FileServiceServer struct {
	pb.UnimplementedFileServiceServer
	l   *logger.Logger
	cfg *config.Config
}

// Init the service execution context
func New(l *logger.Logger, cfg *config.Config) *FileServiceServer {
	return &FileServiceServer{
		l:   l,
		cfg: cfg,
	}
}

// Handler of file upload requests
// Manages large files upload by supporting the streaming of data chunks
func (g *FileServiceServer) Upload(stream pb.FileService_UploadServer) error {
	file := NewFile()
	var fileSize uint32
	fileSize = 0
	defer func() {
		if err := file.OutputFile.Close(); err != nil {
			g.l.Error("Failed to close output file '%v'\nError: %v", file.FilePath, err)
		}
	}()

	// Data stream handling
	for {
		req, err := stream.Recv()
		if file.FilePath == "" {
			bucketFilePath := g.computeBucketFilePath(req.GetBucketId())
			file.SetFile(req.GetFileName(), bucketFilePath)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return g.logError(status.Error(codes.Internal, err.Error()))
		}
		chunk := req.GetChunk()
		fileSize += uint32(len(chunk))
		g.l.Debug("Received a chunk with size: %d", fileSize)
		if err := file.Write(chunk); err != nil {
			return g.logError(status.Error(codes.Internal, err.Error()))
		}
	}

	fmt.Println(file.FilePath, fileSize)
	fileName := filepath.Base(file.FilePath)
	g.l.Info("Saved file: %s, size: %d", fileName, fileSize)

	return stream.SendAndClose(&pb.FileUploadResponse{FileName: fileName, Size: fileSize})
}

func (g *FileServiceServer) computeBucketFilePath(bucketId string) string {
	return g.cfg.FilesStorage.Location + "/" + bucketId
}

// Handle requests for retrieving the file hashes of a given storage bucket
func (g *FileServiceServer) BucketFileHashes(ctx context.Context, req *pb.BucketFileHashesRequest) (*pb.BucketFileHashesResponse, error) {
	bucketFilePath := g.computeBucketFilePath(req.GetBucketId())

	// List the file paths
	filePaths, err := futils.ListDirFilePaths(bucketFilePath)
	if err != nil {
		respErr := fmt.Errorf("failed to list files in bucket '%v' (dir: %v)\n%v", req.GetBucketId(), bucketFilePath, err)
		g.l.Error(fmt.Sprint(respErr))
		return nil, respErr
	}

	// Get the hash for all files
	fileHashes, err := futils.ComputeFileHashes(filePaths)
	if err != nil {
		respErr := fmt.Errorf("failed to compute file hashes for bucket '%v' (dir: %v)\n%v", req.GetBucketId(), bucketFilePath, err)
		g.l.Error(fmt.Sprint(respErr))
		return nil, respErr
	}

	if g.cfg.Log.Level == "debug" {
		for i := 0; i < len(fileHashes); i++ {
			//g.l.Debug("Computed hash for file '%v' : %v", filepath.Base(filePaths[i]), fileHashes[i])
			fmt.Printf("Computed hash for file %d '%v' : %v\n", i, filepath.Base(filePaths[i]), fileHashes[i])
		}
	}

	return &pb.BucketFileHashesResponse{FileHashes: fileHashes}, nil
}

// Handle file download requests
func (g *FileServiceServer) Download(req *pb.FileDownloadRequest, server pb.FileService_DownloadServer) error {
	fileIndex := int(req.GetFileIndex())
	bucketId := req.GetBucketId()
	if bucketId == "" || fileIndex < 0 {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("Valid bucket ID (%v) and file index (%d) are required", bucketId, fileIndex))
	}
	bucketFilePath := g.computeBucketFilePath(bucketId)
	filePaths, err := futils.ListDirFilePaths(bucketFilePath)
	if err != nil {
		respMsg := fmt.Sprintf("Could not find files in '%v'\n%v", bucketFilePath, err)
		g.l.Warn(respMsg)
		return status.Error(codes.NotFound, respMsg)
	}
	if fileIndex > len(filePaths) {
		respMsg := fmt.Sprintf("File index is out of range for bucket '%v' (%d)\n%v", bucketFilePath, len(filePaths), err)
		g.l.Warn(respMsg)
		return status.Error(codes.NotFound, respMsg)
	}

	srcFile, err := getFile(filePaths, fileIndex)
	if err != nil {
		respMsg := fmt.Sprintf("Unsupported file at index %d - local path: %v\n%v", fileIndex, filePaths[fileIndex], err)
		g.l.Warn(respMsg)
		return status.Error(codes.NotFound, respMsg)
	}

	err = server.SendHeader(srcFile.Metadata())
	if err != nil {
		respMsg := fmt.Sprintf("error during sending file header of %v \n%v", srcFile.Name, err)
		g.l.Warn(respMsg)
		return status.Error(codes.Internal, respMsg)
	}

	const chunkSize = 1024 * 3
	chunk := &pb.FileDownloadResponse{Chunk: make([]byte, chunkSize)}
	var n int
Loop:
	for {
		n, err = srcFile.Read(chunk.Chunk)
		switch err {
		case nil:
		case io.EOF:
				break Loop
		default:
				return status.Errorf(codes.Internal, "failed transfering file chunk - io.ReadAll on %v: %v", srcFile.Name, err)
		}
		chunk.Chunk = chunk.Chunk[:n]
		serverErr := server.Send(chunk)
		if serverErr != nil {
				return status.Errorf(codes.Internal, "failed transfering file chunk - server.Send on %v: %v", srcFile.Name, serverErr)
		}
	}

	return nil
}

func getFile(filePaths []string, fileIndex int) (*file.File, error) {
	filePath := filePaths[fileIndex]
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	fileName := filepath.Base(filePath)
	return file.NewFile(fileName, filepath.Ext(fileName), len(fileData), bytes.NewReader(fileData)), nil
}
