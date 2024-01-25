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

	config "github.com/ja88a/vrfs-go-merkletree/libs/config"
	logger "github.com/ja88a/vrfs-go-merkletree/libs/logger"
	mtutils "github.com/ja88a/vrfs-go-merkletree/libs/merkletree/utils"
	rpcfile "github.com/ja88a/vrfs-go-merkletree/libs/rpcapi/file"
	pb "github.com/ja88a/vrfs-go-merkletree/libs/rpcapi/protos/v1/vrfs-fs"
)

// Execution context of the service
type FileStorageService struct {
	pb.UnimplementedFileServiceServer
	l   *logger.Logger
	cfg *config.Config
}

// Init the service execution context
func New(l *logger.Logger, cfg *config.Config) *FileStorageService {
	return &FileStorageService{
		l:   l,
		cfg: cfg,
	}
}

// Upload is the method for handling the file upload requests
// Manages large files upload by supporting the streaming of data chunks
func (g *FileStorageService) Upload(stream pb.FileService_UploadServer) error {
	file := NewFile()
	var fileSize uint32
	fileSize = 0

	// Ensure the file closure in case of interruptions
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
		//g.l.Debug("Received a chunk with size: %d", fileSize)
		if err := file.Write(chunk); err != nil {
			return g.logError(status.Error(codes.Internal, err.Error()))
		}
	}

	fileName := filepath.Base(file.FilePath)
	g.l.Debug("Saved file: %s, size: %d", file.FilePath, fileSize)

	return stream.SendAndClose(&pb.FileUploadResponse{FileName: fileName, Size: fileSize})
}

// Utility method for computing the file path of a storage bucket out of its ID
func (g *FileStorageService) computeBucketFilePath(bucketId string) string {
	return g.cfg.FilesStorage.Location + "/" + bucketId
}

// BucketFileHashes is a method for handling the requests for retrieving the file hashes of a given storage bucket
func (g *FileStorageService) BucketFileHashes(ctx context.Context, req *pb.BucketFileHashesRequest) (*pb.BucketFileHashesResponse, error) {
	g.l.Info("Handle request for the file hashes of bucket '%v'", req.GetBucketId())
	bucketFilePath := g.computeBucketFilePath(req.GetBucketId())

	// List the file paths
	filePaths, err := mtutils.ListDirFilePaths(bucketFilePath)
	if err != nil {
		respErr := fmt.Errorf("failed to list files in bucket '%v' (dir: %v)\n%v", req.GetBucketId(), bucketFilePath, err)
		g.l.Error(fmt.Sprint(respErr))
		return nil, respErr
	}

	// Compute the hash for each file
	fileHashes, err := mtutils.ComputeFileHashes(filePaths)
	if err != nil {
		respErr := fmt.Errorf("failed to compute file hashes for bucket '%v' (dir: %v)\n%v", req.GetBucketId(), bucketFilePath, err)
		g.l.Error(fmt.Sprint(respErr))
		return nil, respErr
	}

	// Debugging
	// if g.cfg.Log.Level == "debug" {
	// 	sHashList := fmt.Sprintf("Computed file hashes for bucket '%v' : \n", bucketFilePath)
	// 	for i := 0; i < len(fileHashes); i++ {
	// 		sHashList += fmt.Sprintf("\nFile %3d Hash: %x File: %v\n", i, fileHashes[i], filepath.Base(filePaths[i]))
	// 	}
	// 	g.l.Debug(sHashList)
	// }

	return &pb.BucketFileHashesResponse{FileHashes: fileHashes}, nil
}

// Download is the method for handling a request for a file download
func (g *FileStorageService) Download(req *pb.FileDownloadRequest, server pb.FileService_DownloadServer) error {
	g.l.Info("Handle download request: bucket '%v' file #%d", req.GetBucketId(), req.FileIndex)

	fileIndex := int(req.GetFileIndex())
	bucketId := req.GetBucketId()
	if bucketId == "" || fileIndex < 0 {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("Valid bucket ID (%v) and file index (%d) are required", bucketId, fileIndex))
	}
	bucketFilePath := g.computeBucketFilePath(bucketId)
	filePaths, err := mtutils.ListDirFilePaths(bucketFilePath)
	if err != nil {
		respMsg := fmt.Sprintf("No files found in '%v'\n%v", bucketFilePath, err)
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
		respMsg := fmt.Sprintf("error while sending the file header of %v \n%v", srcFile.Name, err)
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

// getFile is an internal method for converting locally stored files into a streamable content 
// with corresponding metadata, for clients' file download operations
func getFile(filePaths []string, fileIndex int) (*rpcfile.File, error) {
	filePath := filePaths[fileIndex]
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	fileName := filepath.Base(filePath)
	return rpcfile.NewFile(fileName, filepath.Ext(fileName), len(fileData), bytes.NewReader(fileData)), nil
}
