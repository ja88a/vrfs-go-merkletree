package file

import (
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestMetadataRoundTrip(t *testing.T) {
	for _, name := range []string{"plain.txt", "café.txt", "日本語.データ", "with space.txt", "100%.txt", "line\nbreak.txt"} {
		t.Run(name, func(t *testing.T) {
			md := NewFile(name, ".ext", 3, nil).Metadata()
			for _, value := range md.Get(HeaderFileName) {
				for _, c := range []byte(value) {
					if c < 0x20 || c > 0x7E {
						t.Fatalf("header %q is not printable ASCII", value)
					}
				}
			}

			got := NewFromMetadata(md, nil)
			if got.Name != name || got.Extension != ".ext" || got.Size != 3 {
				t.Fatalf("got %+v, want name %q", got, name)
			}
		})
	}
}

// An encoded separator must decode to an unsafe name, so that receivers refuse it.
func TestEncodedPathTraversalIsRefusedOnceDecoded(t *testing.T) {
	md := metadata.New(map[string]string{HeaderFileName: "..%2F..%2Fescaped.txt"})

	if name := NewFromMetadata(md, nil).Name; IsSafeName(name) {
		t.Fatalf("decoded name %q must be unsafe", name)
	}
}

func TestMalformedHeaderDecodesToUnsafeName(t *testing.T) {
	md := metadata.New(map[string]string{HeaderFileName: "bad%zz.txt"})

	if name := NewFromMetadata(md, nil).Name; IsSafeName(name) {
		t.Fatalf("malformed name decoded to %q", name)
	}
}
