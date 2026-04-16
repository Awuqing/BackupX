package backint

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseBackupRequests(t *testing.T) {
	input := `#PIPE /tmp/pipe1
#PIPE "/tmp/pipe two"
/tmp/file.bak
"/tmp/file two.bak"
`
	reqs, err := ParseBackupRequests(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(reqs) != 4 {
		t.Fatalf("expected 4 requests, got %d", len(reqs))
	}
	if !reqs[0].IsPipe || reqs[0].Path != "/tmp/pipe1" {
		t.Errorf("req[0] mismatch: %+v", reqs[0])
	}
	if !reqs[1].IsPipe || reqs[1].Path != "/tmp/pipe two" {
		t.Errorf("req[1] mismatch: %+v", reqs[1])
	}
	if reqs[2].IsPipe || reqs[2].Path != "/tmp/file.bak" {
		t.Errorf("req[2] mismatch: %+v", reqs[2])
	}
	if reqs[3].Path != "/tmp/file two.bak" {
		t.Errorf("req[3] mismatch: %+v", reqs[3])
	}
}

func TestParseRestoreRequests(t *testing.T) {
	input := `#PIPE backupx-123 "/tmp/pipe1"
#EBID "backupx-456" "/tmp/file.bak"
backupx-789 /tmp/plain.bak
`
	reqs, err := ParseRestoreRequests(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(reqs) != 3 {
		t.Fatalf("expected 3, got %d", len(reqs))
	}
	if !reqs[0].IsPipe || reqs[0].EBID != "backupx-123" || reqs[0].Path != "/tmp/pipe1" {
		t.Errorf("req[0] mismatch: %+v", reqs[0])
	}
	if reqs[1].IsPipe || reqs[1].EBID != "backupx-456" {
		t.Errorf("req[1] mismatch: %+v", reqs[1])
	}
	if reqs[2].EBID != "backupx-789" || reqs[2].Path != "/tmp/plain.bak" {
		t.Errorf("req[2] mismatch: %+v", reqs[2])
	}
}

func TestParseInquireRequests(t *testing.T) {
	input := "#NULL\nbackupx-abc\n#EBID \"backupx-xyz\"\n"
	reqs, err := ParseInquireRequests(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(reqs) != 3 {
		t.Fatalf("expected 3, got %d", len(reqs))
	}
	if !reqs[0].All {
		t.Errorf("req[0] should be All")
	}
	if reqs[1].EBID != "backupx-abc" {
		t.Errorf("req[1] mismatch: %+v", reqs[1])
	}
	if reqs[2].EBID != "backupx-xyz" {
		t.Errorf("req[2] mismatch: %+v", reqs[2])
	}
}

func TestParseDeleteRequests(t *testing.T) {
	input := "backupx-aaa\n#EBID \"backupx-bbb\"\n"
	reqs, err := ParseDeleteRequests(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(reqs) != 2 || reqs[0].EBID != "backupx-aaa" || reqs[1].EBID != "backupx-bbb" {
		t.Fatalf("unexpected: %+v", reqs)
	}
}

func TestWriteResponses(t *testing.T) {
	var buf bytes.Buffer
	_ = WriteSaved(&buf, "backupx-1", "/tmp/x")
	_ = WriteRestored(&buf, "backupx-2", "/tmp/y")
	_ = WriteBackup(&buf, "backupx-3")
	_ = WriteNotFound(&buf, "backupx-4")
	_ = WriteDeleted(&buf, "backupx-5")
	_ = WriteError(&buf, "/tmp/z")
	want := "#SAVED backupx-1 \"/tmp/x\"\n" +
		"#RESTORED \"backupx-2\" \"/tmp/y\"\n" +
		"#BACKUP \"backupx-3\"\n" +
		"#NOTFOUND \"backupx-4\"\n" +
		"#DELETED \"backupx-5\"\n" +
		"#ERROR \"/tmp/z\"\n"
	if buf.String() != want {
		t.Errorf("output mismatch:\n got: %q\nwant: %q", buf.String(), want)
	}
}

func TestParseFunction(t *testing.T) {
	cases := map[string]Function{
		"backup":  FunctionBackup,
		"BACKUP":  FunctionBackup,
		"restore": FunctionRestore,
		"inquire": FunctionInquire,
		"delete":  FunctionDelete,
	}
	for s, want := range cases {
		got, err := ParseFunction(s)
		if err != nil || got != want {
			t.Errorf("ParseFunction(%q) = %v, %v; want %v", s, got, err, want)
		}
	}
	if _, err := ParseFunction("bogus"); err == nil {
		t.Errorf("expected error for bogus function")
	}
}

func TestSplitFirstField(t *testing.T) {
	cases := []struct{ in, first, rest string }{
		{`abc def`, "abc", "def"},
		{`"abc def" ghi`, "abc def", "ghi"},
		{`"a b" "c d"`, "a b", `"c d"`},
		{`lone`, "lone", ""},
		{``, "", ""},
	}
	for _, c := range cases {
		f, r := splitFirstField(c.in)
		if f != c.first || r != c.rest {
			t.Errorf("splitFirstField(%q) = (%q, %q); want (%q, %q)", c.in, f, r, c.first, c.rest)
		}
	}
}
