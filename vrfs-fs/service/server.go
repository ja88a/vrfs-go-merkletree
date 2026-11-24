package service

import (
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

// Directory of the storage location holding the uploads in progress. Being
// on the same filesystem as the buckets makes moving a completed file atomic.
const uploadTmpDirName = ".uploads-tmp"

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
//
// The file is stored, and the upload acknowledged, only once the complete stream has been written
func (g *FileStorageService) Upload(stream pb.FileService_UploadServer) error {
	file := NewFile()
	var bucketId, fileName string
	var fileSize uint32

	// Ensure that an interrupted upload leaves nothing behind
	defer func() {
		if err := file.Discard(); err != nil {
			g.l.Error("Failed to discard the temporary upload of '%v'\nError: %v", file.FilePath, err)
		}
	}()

	// Data stream handling
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return g.logError(status.Error(codes.Internal, err.Error()))
		}

		if file.OutputFile == nil {
			bucketId, fileName = req.GetBucketId(), req.GetFileName()
			if err := validateBucketId(bucketId); err != nil {
				return g.logError(err)
			}
			if !rpcfile.IsSafeName(fileName) {
				return g.logError(status.Errorf(codes.InvalidArgument, "invalid file name %q", fileName))
			}
			tmpDir := filepath.Join(g.cfg.FilesStorage.Location, uploadTmpDirName)
			if err := file.SetFile(fileName, g.computeBucketFilePath(bucketId), tmpDir); err != nil {
				return g.logError(status.Errorf(codes.Internal, "failed to store file %q: %v", fileName, err))
			}
		} else if req.GetBucketId() != bucketId || req.GetFileName() != fileName {
			return g.logError(status.Error(codes.InvalidArgument, "bucket ID and file name must not change during an upload"))
		}

		chunk := req.GetChunk()
		fileSize += uint32(len(chunk))
		//g.l.Debug("Received a chunk with size: %d", fileSize)
		if err := file.Write(chunk); err != nil {
			return g.logError(status.Error(codes.Internal, err.Error()))
		}
	}

	if file.OutputFile == nil {
		return g.logError(status.Error(codes.InvalidArgument, "upload stream without file metadata"))
	}
	if err := file.Commit(); err != nil {
		return g.logError(status.Errorf(codes.Internal, "failed to store file %q: %v", fileName, err))
	}
	g.l.Debug("Saved file: %s, size: %d", file.FilePath, fileSize)

	return stream.SendAndClose(&pb.FileUploadResponse{FileName: fileName, Size: fileSize})
}

// Check that a client-provided bucket ID designates a directory of the storage location
func validateBucketId(bucketId string) error {
	if !rpcfile.IsSafeName(bucketId) || bucketId == uploadTmpDirName {
		return status.Errorf(codes.InvalidArgument, "invalid bucket ID %q", bucketId)
	}
	return nil
}

// Utility method for computing the file path of a storage bucket out of its ID
func (g *FileStorageService) computeBucketFilePath(bucketId string) string {
	return g.cfg.FilesStorage.Location + "/" + bucketId
}

// BucketFileHashes is a method for handling the requests for retrieving the file hashes of a given storage bucket
func (g *FileStorageService) BucketFileHashes(ctx context.Context, req *pb.BucketFileHashesRequest) (*pb.BucketFileHashesResponse, error) {
	g.l.Info("Handle request for the file hashes of bucket '%v'", req.GetBucketId())
	if err := validateBucketId(req.GetBucketId()); err != nil {
		return nil, g.logError(err)
	}
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
	if err := validateBucketId(bucketId); err != nil {
		return g.logError(err)
	}
	if fileIndex < 0 {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("Valid file index (%d) is required", fileIndex))
	}
	bucketFilePath := g.computeBucketFilePath(bucketId)
	filePaths, err := mtutils.ListDirFilePaths(bucketFilePath)
	if err != nil {
		respMsg := fmt.Sprintf("No files found in '%v'\n%v", bucketFilePath, err)
		g.l.Warn(respMsg)
		return status.Error(codes.NotFound, respMsg)
	}
	if fileIndex >= len(filePaths) {
		respMsg := fmt.Sprintf("File index %d is out of range for bucket '%v' (%d files)", fileIndex, bucketId, len(filePaths))
		g.l.Warn(respMsg)
		return status.Error(codes.NotFound, respMsg)
	}

	srcFile, err := getFile(filePaths, fileIndex)
	if err != nil {
		respMsg := fmt.Sprintf("Unsupported file at index %d - local path: %v\n%v", fileIndex, filePaths[fileIndex], err)
		g.l.Warn(respMsg)
		return status.Error(codes.NotFound, respMsg)
	}
	defer srcFile.Close()

	err = server.SendHeader(srcFile.Metadata())
	if err != nil {
		respMsg := fmt.Sprintf("error while sending the file header of %v \n%v", srcFile.Name, err)
		g.l.Warn(respMsg)
		return status.Error(codes.Internal, respMsg)
	}

	const chunkSize = 1024 * 3
	buf := make([]byte, chunkSize)
	for {
		n, err := srcFile.Read(buf)
		if n > 0 {
			if serverErr := server.Send(&pb.FileDownloadResponse{Chunk: buf[:n]}); serverErr != nil {
				return status.Errorf(codes.Internal, "failed transfering file chunk - server.Send on %v: %v", srcFile.Name, serverErr)
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return status.Errorf(codes.Internal, "failed transfering file chunk - read on %v: %v", srcFile.Name, err)
		}
	}
}

// getFile is an internal method for opening a locally stored file as a streamable content
// with corresponding metadata, for clients' file download operations.
// Its data is read on demand, so that the response headers do not wait for the whole file.
func getFile(filePaths []string, fileIndex int) (*rpcfile.File, error) {
	filePath := filePaths[fileIndex]
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	fileName := filepath.Base(filePath)
	return rpcfile.NewFile(fileName, filepath.Ext(fileName), int(info.Size()), file), nil
}
