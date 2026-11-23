package app

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mt "github.com/ja88a/vrfs-go-merkletree/libs/merkletree"
	mtutils "github.com/ja88a/vrfs-go-merkletree/libs/merkletree/utils"
	rpcfile "github.com/ja88a/vrfs-go-merkletree/libs/rpcapi/file"
)

// downloadFixture holds a real two-file Merkle tree and the expected first file.
type downloadFixture struct {
	fileSetID string
	name      string
	content   []byte
	proofs    []*mt.Proof
}

func newDownloadFixture(t *testing.T) downloadFixture {
	t.Helper()
	dir := t.TempDir()
	paths := writeFiles(t, dir, 2)
	hashes, err := mtutils.ComputeFileHashes(paths)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := mtutils.GenerateMerkleTree(hashes, true)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	return downloadFixture{
		fileSetID: FilesetNamePrefix + hex.EncodeToString(tree.Root),
		name:      filepath.Base(paths[0]),
		content:   content,
		proofs:    tree.Proofs,
	}
}

func (f downloadFixture) client(name string, content []byte) *ClientContext {
	return &ClientContext{
		Vrfs: &fakeVrfs{proof: f.proofs[0]},
		Nfs: &fakeTransfer{download: func(string, int) (*rpcfile.File, error) {
			return rpcfile.NewFile(name, filepath.Ext(name), len(content), bytes.NewReader(content)), nil
		}},
	}
}

func TestDownloadFileAcceptsFirstIndexAndVerifies(t *testing.T) {
	fixture := newDownloadFixture(t)
	downDir := t.TempDir()

	if err := fixture.client(fixture.name, fixture.content).DownloadFile(fixture.fileSetID, 0, downDir); err != nil {
		t.Fatalf("index 0 must be downloadable: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(downDir, fixture.fileSetID, fixture.name))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, fixture.content) {
		t.Fatalf("content = %q, want %q", got, fixture.content)
	}
	assertOnlyEntry(t, filepath.Join(downDir, fixture.fileSetID), fixture.name)
}

func TestDownloadFileLeavesNothingWhenVerificationFails(t *testing.T) {
	fixture := newDownloadFixture(t)
	downDir := t.TempDir()

	err := fixture.client(fixture.name, []byte("tampered")).DownloadFile(fixture.fileSetID, 0, downDir)
	if err == nil || !strings.Contains(err.Error(), "fails the verification") {
		t.Fatalf("expected tampered content to fail verification, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(downDir, fixture.fileSetID, fixture.name)); !os.IsNotExist(statErr) {
		t.Fatalf("tampered file must not be left at its final path (stat: %v)", statErr)
	}
	assertOnlyEntry(t, filepath.Join(downDir, fixture.fileSetID))
}

func TestDownloadFileKeepsVerifiedCopyWhenNewCopyIsTampered(t *testing.T) {
	fixture := newDownloadFixture(t)
	downDir := t.TempDir()

	if err := fixture.client(fixture.name, fixture.content).DownloadFile(fixture.fileSetID, 0, downDir); err != nil {
		t.Fatal(err)
	}
	if err := fixture.client(fixture.name, []byte("tampered")).DownloadFile(fixture.fileSetID, 0, downDir); err == nil {
		t.Fatal("expected tampered content to fail verification")
	}
	got, err := os.ReadFile(filepath.Join(downDir, fixture.fileSetID, fixture.name))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, fixture.content) {
		t.Fatalf("verified copy was overwritten with %q", got)
	}
}

func TestDownloadFileRejectsUnsafeServerFileName(t *testing.T) {
	fixture := newDownloadFixture(t)
	root := t.TempDir()
	downDir := filepath.Join(root, "downloads")

	for _, name := range []string{"../../escaped.txt", "..", "", "nested/file.txt"} {
		t.Run(name, func(t *testing.T) {
			err := fixture.client(name, fixture.content).DownloadFile(fixture.fileSetID, 0, downDir)
			if err == nil || !strings.Contains(err.Error(), "unsafe file name") {
				t.Fatalf("server-supplied file name %q must be rejected, got: %v", name, err)
			}
			if _, err := os.Stat(filepath.Join(root, "escaped.txt")); !os.IsNotExist(err) {
				t.Fatalf("file escaped the download directory (stat: %v)", err)
			}
		})
	}
}

func assertOnlyEntry(t *testing.T, dir string, want ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) && len(want) == 0 {
			return
		}
		t.Fatal(err)
	}
	if len(entries) != len(want) {
		t.Fatalf("directory %s has %d entries, want %v", dir, len(entries), want)
	}
	for i, entry := range entries {
		if entry.Name() != want[i] {
			t.Fatalf("unexpected entry %q in %s", entry.Name(), dir)
		}
	}
}
