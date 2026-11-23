package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ja88a/vrfs-go-merkletree/client/rservice"
	mt "github.com/ja88a/vrfs-go-merkletree/libs/merkletree"
	rpcfile "github.com/ja88a/vrfs-go-merkletree/libs/rpcapi/file"
)

// fakeTransfer is an in-memory rservice.FTService.
type fakeTransfer struct {
	upload   func(bucketID, filePath string) error
	download func(bucketID string, fileIndex int) (*rpcfile.File, error)
}

func (f *fakeTransfer) UploadFile(bucketID, filePath string) error {
	return f.upload(bucketID, filePath)
}

func (f *fakeTransfer) DownloadFile(bucketID string, fileIndex int) (*rpcfile.File, error) {
	return f.download(bucketID, fileIndex)
}

// fakeVrfs is an in-memory rservice.VrfsService.
type fakeVrfs struct {
	proof      *mt.Proof
	doneStatus int32
	doneCalls  atomic.Int32
}

func (f *fakeVrfs) HandleFileBucketReq(_, fileSetID string) (int32, string, error) {
	return 0, "bucket-" + fileSetID, nil
}

func (f *fakeVrfs) HandleUploadDoneReq(_, _ string, _ []byte) (int32, string, error) {
	f.doneCalls.Add(1)
	return f.doneStatus, "done", nil
}

func (f *fakeVrfs) HandleDownloadFileInfoReq(_, fileSetID string, _ int) (string, *mt.Proof, error) {
	return "bucket-" + fileSetID, f.proof, nil
}

func (f *fakeVrfs) HandlePingReq() error { return nil }

func writeFiles(t *testing.T, dir string, count int) []string {
	t.Helper()
	paths := make([]string, 0, count)
	for i := 0; i < count; i++ {
		path := filepath.Join(dir, fmt.Sprintf("file-%02d.txt", i))
		if err := os.WriteFile(path, []byte(fmt.Sprintf("content %d", i)), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	return paths
}

func TestUploadFilesConcurrentlyWaitsForEveryUpload(t *testing.T) {
	paths := writeFiles(t, t.TempDir(), 6)
	var completed atomic.Int32
	ctx := &ClientContext{Nfs: &fakeTransfer{upload: func(string, string) error {
		time.Sleep(40 * time.Millisecond)
		completed.Add(1)
		return nil
	}}}

	if err := ctx.uploadFilesConcurrently("bucket", paths, 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := completed.Load(); got != int32(len(paths)) {
		t.Fatalf("returned after %d of %d uploads completed", got, len(paths))
	}
}

func TestUploadFilesConcurrentlyRespectsLimit(t *testing.T) {
	paths := writeFiles(t, t.TempDir(), 12)
	var active, peak atomic.Int32
	ctx := &ClientContext{Nfs: &fakeTransfer{upload: func(string, string) error {
		now := active.Add(1)
		for {
			seen := peak.Load()
			if now <= seen || peak.CompareAndSwap(seen, now) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		active.Add(-1)
		return nil
	}}}

	if err := ctx.uploadFilesConcurrently("bucket", paths, 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := peak.Load(); got > 3 {
		t.Fatalf("peak concurrency %d exceeds the limit of 3", got)
	}
}

func TestUploadFilesConcurrentlyReportsEveryFailure(t *testing.T) {
	paths := writeFiles(t, t.TempDir(), 4)
	ctx := &ClientContext{Nfs: &fakeTransfer{upload: func(_, path string) error {
		time.Sleep(20 * time.Millisecond)
		if strings.HasSuffix(path, "01.txt") || strings.HasSuffix(path, "03.txt") {
			return errors.New("storage refused " + filepath.Base(path))
		}
		return nil
	}}}

	err := ctx.uploadFilesConcurrently("bucket", paths, 2)
	if err == nil {
		t.Fatal("expected the failed uploads to be reported")
	}
	for _, name := range []string{"file-01.txt", "file-03.txt"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error does not mention %s: %v", name, err)
		}
	}
}

func TestUploadFilesConcurrentlyStopsAfterInterruption(t *testing.T) {
	paths := writeFiles(t, t.TempDir(), 6)
	var started atomic.Int32
	ctx := &ClientContext{Nfs: &fakeTransfer{upload: func(string, string) error {
		started.Add(1)
		return fmt.Errorf("upload stopped: %w", rservice.ErrUploadInterrupted)
	}}}

	err := ctx.uploadFilesConcurrently("bucket", paths, 1)
	if !errors.Is(err, rservice.ErrUploadInterrupted) {
		t.Fatalf("expected the interruption to be reported, got: %v", err)
	}
	if got := started.Load(); got != 1 {
		t.Fatalf("%d uploads started, want 1", got)
	}
}

func TestUploadFilesetKeepsLocalFilesWhenAnUploadFails(t *testing.T) {
	dir := t.TempDir()
	paths := writeFiles(t, dir, 3)
	vrfs := &fakeVrfs{doneStatus: 200}
	ctx := &ClientContext{Vrfs: vrfs, Nfs: &fakeTransfer{upload: func(_, path string) error {
		time.Sleep(20 * time.Millisecond)
		if path == paths[1] {
			return errors.New("connection lost")
		}
		return nil
	}}}

	if err := ctx.UploadFileset(dir, 2); err == nil {
		t.Fatal("expected the fileset upload to fail")
	}
	if got := vrfs.doneCalls.Load(); got != 0 {
		t.Fatalf("upload was confirmed to VRFS %d time(s) despite a failure", got)
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("local file must be kept after a failed upload: %v", err)
		}
	}
}

func TestUploadFilesetKeepsLocalFilesOnRootMismatch(t *testing.T) {
	dir := t.TempDir()
	paths := writeFiles(t, dir, 3)
	ctx := &ClientContext{
		Vrfs: &fakeVrfs{doneStatus: 419},
		Nfs:  &fakeTransfer{upload: func(string, string) error { return nil }},
	}

	if err := ctx.UploadFileset(dir, 2); err == nil {
		t.Fatal("expected a root mismatch to fail the upload")
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("local file must be kept after a mismatch: %v", err)
		}
	}
}
