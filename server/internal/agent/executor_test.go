package agent

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"backupx/server/internal/storage"
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

func TestExecuteRunTaskRecordsPerTargetUploadResults(t *testing.T) {
	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "index.html"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	var finalUpdate RecordUpdate
	var updates []RecordUpdate
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/agent/tasks/1":
			writeAgentEnvelope(t, w, TaskSpec{
				TaskID:      1,
				Name:        "site",
				Type:        "file",
				SourcePath:  sourceDir,
				Compression: "gzip",
				StorageTargets: []StorageTargetConfig{
					{ID: 11, Name: "broken", Type: "agent_test_storage", Config: json.RawMessage(`{"name":"broken"}`)},
					{ID: 12, Name: "ok", Type: "agent_test_storage", Config: json.RawMessage(`{"name":"ok"}`)},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/agent/records/99":
			var update RecordUpdate
			if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
				t.Fatalf("Decode update returned error: %v", err)
			}
			updates = append(updates, update)
			if update.Status != "" {
				finalUpdate = update
			}
			writeAgentEnvelope(t, w, map[string]string{"status": "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	executor := NewExecutor(NewMasterClient(server.URL, "token", false), filepath.Join(t.TempDir(), "tmp"))
	executor.storageRegistry = storage.NewRegistry(&agentTestStorageFactory{
		providers: map[string]*agentTestStorageProvider{
			"broken": {name: "broken", failUpload: true},
			"ok":     {name: "ok", objects: map[string][]byte{}},
		},
	})

	if err := executor.ExecuteRunTask(context.Background(), 1, 99); err != nil {
		t.Fatalf("ExecuteRunTask returned error: %v", err)
	}
	if len(updates) == 0 || finalUpdate.Status != "success" {
		t.Fatalf("expected final success update, got updates=%#v final=%#v", updates, finalUpdate)
	}
	if finalUpdate.StorageTargetID != 12 {
		t.Fatalf("expected first successful target 12, got %d", finalUpdate.StorageTargetID)
	}
	if len(finalUpdate.StorageUploadResults) != 2 {
		t.Fatalf("expected two upload results, got %#v", finalUpdate.StorageUploadResults)
	}
	if finalUpdate.StorageUploadResults[0].Status != "failed" || finalUpdate.StorageUploadResults[1].Status != "success" {
		t.Fatalf("unexpected upload results: %#v", finalUpdate.StorageUploadResults)
	}
	if finalUpdate.StoragePath == "" || finalUpdate.FileSize <= 0 || finalUpdate.Checksum == "" {
		t.Fatalf("expected artifact metadata in final update, got %#v", finalUpdate)
	}
}

func TestExecuteRunTaskRelaysMasterLocalDiskTarget(t *testing.T) {
	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "index.html"), []byte("centralize me"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	var relayed []byte
	var finalUpdate RecordUpdate
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/agent/tasks/1":
			writeAgentEnvelope(t, w, TaskSpec{
				TaskID:      1,
				Name:        "remote-source",
				Type:        "file",
				SourcePath:  sourceDir,
				Compression: "gzip",
				StorageTargets: []StorageTargetConfig{{
					ID: 11, Name: "master-disk", Type: storage.TypeLocalDisk, TransferMode: storage.TransferModeMasterRelay,
				}},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/api/agent/records/99/artifacts/11":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll relayed body: %v", err)
			}
			digest := sha256.Sum256(body)
			if got := r.Header.Get("X-BackupX-SHA256"); got != fmt.Sprintf("%x", digest[:]) {
				t.Fatalf("relay checksum header = %q", got)
			}
			objectKey := r.Header.Get("X-BackupX-Object-Key")
			if !strings.Contains(objectKey, "/records/99/") || r.ContentLength != int64(len(body)) {
				t.Fatalf("invalid relay metadata: key=%q length=%d body=%d", objectKey, r.ContentLength, len(body))
			}
			relayed = append([]byte(nil), body...)
			writeAgentEnvelope(t, w, map[string]string{"status": "ok"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/agent/records/99":
			var update RecordUpdate
			if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
				t.Fatalf("Decode update returned error: %v", err)
			}
			if update.Status != "" {
				finalUpdate = update
			}
			writeAgentEnvelope(t, w, map[string]string{"status": "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	executor := NewExecutor(NewMasterClient(server.URL, "token", false), filepath.Join(t.TempDir(), "tmp"))
	if err := executor.ExecuteRunTask(context.Background(), 1, 99); err != nil {
		t.Fatalf("ExecuteRunTask returned error: %v", err)
	}
	if len(relayed) == 0 {
		t.Fatal("expected artifact bytes to be streamed through Master")
	}
	if finalUpdate.Status != "success" || finalUpdate.StorageTransferMode != storage.TransferModeMasterRelay {
		t.Fatalf("unexpected final relay update: %#v", finalUpdate)
	}
	if len(finalUpdate.StorageUploadResults) != 1 || finalUpdate.StorageUploadResults[0].TransferMode != storage.TransferModeMasterRelay {
		t.Fatalf("unexpected relay target result: %#v", finalUpdate.StorageUploadResults)
	}
}

func TestExecuteRestoreDownloadsMasterRelayedArtifact(t *testing.T) {
	var archive bytes.Buffer
	tarWriter := tar.NewWriter(&archive)
	content := []byte("restored through Master")
	header := &tar.Header{Name: "site/index.html", Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("WriteHeader returned error: %v", err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	artifact := append([]byte(nil), archive.Bytes()...)
	digest := sha256.Sum256(artifact)
	restoreRoot := t.TempDir()
	restoreSource := filepath.Join(restoreRoot, "site")
	artifactRequests := 0
	var finalUpdate RestoreUpdate

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/agent/restores/77/spec":
			writeAgentEnvelope(t, w, RestoreSpec{
				RestoreRecordID: 77,
				BackupRecordID:  99,
				TaskID:          1,
				TaskName:        "remote-source",
				Type:            "file",
				SourcePath:      restoreSource,
				Storage: StorageTargetConfig{
					ID: 11, Name: "master-disk", Type: storage.TypeLocalDisk, TransferMode: storage.TransferModeMasterRelay,
				},
				StoragePath: "BackupX/file/site.tar",
				FileName:    "site.tar",
				Checksum:    fmt.Sprintf("%x", digest[:]),
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/agent/restores/77/artifact":
			artifactRequests++
			if r.Header.Get("X-Agent-Token") != "token" {
				t.Fatalf("missing Agent token on relay download")
			}
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(artifact)))
			_, _ = w.Write(artifact)
		case r.Method == http.MethodPost && r.URL.Path == "/api/agent/restores/77":
			var update RestoreUpdate
			if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
				t.Fatalf("Decode update returned error: %v", err)
			}
			if update.Status != "" {
				finalUpdate = update
			}
			writeAgentEnvelope(t, w, map[string]string{"status": "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	executor := NewExecutor(NewMasterClient(server.URL, "token", false), filepath.Join(t.TempDir(), "tmp"))
	if err := executor.ExecuteRestore(context.Background(), 77); err != nil {
		t.Fatalf("ExecuteRestore returned error: %v", err)
	}
	if artifactRequests != 1 {
		t.Fatalf("expected one relay artifact request, got %d", artifactRequests)
	}
	restored, err := os.ReadFile(filepath.Join(restoreRoot, "site", "index.html"))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(restored) != string(content) {
		t.Fatalf("restored content = %q, want %q", restored, content)
	}
	if finalUpdate.Status != "success" {
		t.Fatalf("unexpected final restore update: %#v", finalUpdate)
	}
}

func TestExecuteRunTaskReportsPerTargetUploadResultsWhenAllTargetsFail(t *testing.T) {
	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "index.html"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	var finalUpdate RecordUpdate
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/agent/tasks/1":
			writeAgentEnvelope(t, w, TaskSpec{
				TaskID:      1,
				Name:        "site",
				Type:        "file",
				SourcePath:  sourceDir,
				Compression: "gzip",
				StorageTargets: []StorageTargetConfig{
					{ID: 11, Name: "broken-a", Type: "agent_test_storage", Config: json.RawMessage(`{"name":"broken-a"}`)},
					{ID: 12, Name: "broken-b", Type: "agent_test_storage", Config: json.RawMessage(`{"name":"broken-b"}`)},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/agent/records/99":
			var update RecordUpdate
			if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
				t.Fatalf("Decode update returned error: %v", err)
			}
			if update.Status != "" {
				finalUpdate = update
			}
			writeAgentEnvelope(t, w, map[string]string{"status": "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	executor := NewExecutor(NewMasterClient(server.URL, "token", false), filepath.Join(t.TempDir(), "tmp"))
	executor.storageRegistry = storage.NewRegistry(&agentTestStorageFactory{
		providers: map[string]*agentTestStorageProvider{
			"broken-a": {name: "broken-a", failUpload: true},
			"broken-b": {name: "broken-b", failUpload: true},
		},
	})

	if err := executor.ExecuteRunTask(context.Background(), 1, 99); err == nil {
		t.Fatal("expected ExecuteRunTask to return upload failure")
	}
	if finalUpdate.Status != "failed" {
		t.Fatalf("expected final failed update, got %#v", finalUpdate)
	}
	if len(finalUpdate.StorageUploadResults) != 2 {
		t.Fatalf("expected failed update to keep per-target results, got %#v", finalUpdate.StorageUploadResults)
	}
	for _, item := range finalUpdate.StorageUploadResults {
		if item.Status != "failed" || item.Error == "" {
			t.Fatalf("unexpected upload result: %#v", item)
		}
	}
}

type agentTestStorageFactory struct {
	providers map[string]*agentTestStorageProvider
}

func (f *agentTestStorageFactory) Type() storage.ProviderType {
	return "agent_test_storage"
}

func (f *agentTestStorageFactory) New(_ context.Context, config map[string]any) (storage.StorageProvider, error) {
	name, _ := config["name"].(string)
	provider := f.providers[name]
	if provider == nil {
		return nil, fmt.Errorf("unknown provider %q", name)
	}
	return provider, nil
}

type agentTestStorageProvider struct {
	name       string
	failUpload bool
	objects    map[string][]byte
}

func (p *agentTestStorageProvider) Type() storage.ProviderType { return "agent_test_storage" }
func (p *agentTestStorageProvider) TestConnection(context.Context) error {
	return nil
}
func (p *agentTestStorageProvider) Upload(_ context.Context, objectKey string, reader io.Reader, _ int64, _ map[string]string) error {
	if p.failUpload {
		return fmt.Errorf("upload failed for %s", p.name)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	if p.objects == nil {
		p.objects = map[string][]byte{}
	}
	p.objects[objectKey] = data
	return nil
}
func (p *agentTestStorageProvider) Download(_ context.Context, objectKey string) (io.ReadCloser, error) {
	data, ok := p.objects[objectKey]
	if !ok {
		return nil, fmt.Errorf("object %s not found", objectKey)
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}
func (p *agentTestStorageProvider) Delete(_ context.Context, objectKey string) error {
	delete(p.objects, objectKey)
	return nil
}
func (p *agentTestStorageProvider) List(context.Context, string) ([]storage.ObjectInfo, error) {
	return nil, nil
}

func writeAgentEnvelope(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"code": "OK", "data": data}); err != nil {
		t.Fatalf("Encode response returned error: %v", err)
	}
}
