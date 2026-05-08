package agent

import (
	"reflect"
	"testing"
	"time"
)

func TestBuildBackupTaskSpecParsesJSONSourcePaths(t *testing.T) {
	spec := &TaskSpec{
		TaskID:          7,
		Name:            "root-files",
		Type:            "file",
		SourcePaths:     `["/root","/etc"]`,
		ExcludePatterns: `["*.log","tmp"]`,
	}

	got := buildBackupTaskSpec(spec, time.Unix(0, 0), "/var/lib/backupx-agent/tmp")

	if !reflect.DeepEqual(got.SourcePaths, []string{"/root", "/etc"}) {
		t.Fatalf("source paths = %#v", got.SourcePaths)
	}
	if !reflect.DeepEqual(got.ExcludePatterns, []string{"*.log", "tmp"}) {
		t.Fatalf("exclude patterns = %#v", got.ExcludePatterns)
	}
}

func TestParseStringListFieldKeepsLegacyLineFormat(t *testing.T) {
	got := parseStringListField("/root\n /etc \n")
	want := []string{"/root", "/etc"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("paths = %#v, want %#v", got, want)
	}
}
