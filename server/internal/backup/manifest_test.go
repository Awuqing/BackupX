package backup

import (
	"reflect"
	"testing"
)

func TestEncodeDecodeManifestRoundTrip(t *testing.T) {
	m := Manifest{Entries: []ManifestEntry{
		{Path: "src/a.txt", Size: 10, ModTimeNs: 100, Mode: 0o644},
		{Path: "src", Size: 0, ModTimeNs: 50, Mode: 0o755, IsDir: true},
	}}
	data, err := EncodeManifest(m)
	if err != nil {
		t.Fatalf("EncodeManifest: %v", err)
	}
	got, err := DecodeManifest(data)
	if err != nil {
		t.Fatalf("DecodeManifest: %v", err)
	}
	if !reflect.DeepEqual(got, m) {
		t.Fatalf("roundtrip mismatch:\n got %#v\nwant %#v", got, m)
	}
}

func TestDecodeManifestEmpty(t *testing.T) {
	got, err := DecodeManifest(nil)
	if err != nil {
		t.Fatalf("DecodeManifest(nil): %v", err)
	}
	if len(got.Entries) != 0 {
		t.Fatalf("expected empty manifest, got %#v", got)
	}
}

func TestChangedSince(t *testing.T) {
	base := Manifest{Entries: []ManifestEntry{
		{Path: "a.txt", Size: 10, ModTimeNs: 100},
		{Path: "dir", IsDir: true, ModTimeNs: 100},
	}}.index()

	cases := []struct {
		name string
		cur  ManifestEntry
		want bool
	}{
		{"unchanged file", ManifestEntry{Path: "a.txt", Size: 10, ModTimeNs: 100}, false},
		{"size changed", ManifestEntry{Path: "a.txt", Size: 11, ModTimeNs: 100}, true},
		{"mtime changed", ManifestEntry{Path: "a.txt", Size: 10, ModTimeNs: 200}, true},
		{"new file", ManifestEntry{Path: "b.txt", Size: 1, ModTimeNs: 1}, true},
		{"existing dir skipped", ManifestEntry{Path: "dir", IsDir: true, ModTimeNs: 999}, false},
		{"new dir included", ManifestEntry{Path: "newdir", IsDir: true, ModTimeNs: 1}, true},
	}
	for _, tc := range cases {
		if got := changedSince(base, tc.cur); got != tc.want {
			t.Errorf("%s: changedSince=%v want %v", tc.name, got, tc.want)
		}
	}
}

func TestDeletedPaths(t *testing.T) {
	base := Manifest{Entries: []ManifestEntry{
		{Path: "a"}, {Path: "b"}, {Path: "c"},
	}}.index()
	seen := map[string]struct{}{"a": {}, "c": {}}
	got := deletedPaths(base, seen)
	want := []string{"b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deletedPaths=%v want %v", got, want)
	}
}

func TestDeletedPathsNoneWhenAllSeen(t *testing.T) {
	base := Manifest{Entries: []ManifestEntry{{Path: "a"}, {Path: "b"}}}.index()
	seen := map[string]struct{}{"a": {}, "b": {}}
	if got := deletedPaths(base, seen); len(got) != 0 {
		t.Fatalf("expected no deletions, got %v", got)
	}
}
