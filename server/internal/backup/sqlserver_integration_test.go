package backup

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Opt-in only: creates and drops a uniquely named disposable database. Run on a
// dedicated SQL Server test instance, with the real native worker on PATH.
func TestSQLServerVDIRoundTrip(t *testing.T) {
	if os.Getenv("BACKUPX_SQLSERVER_INTEGRATION") != "1" {
		t.Skip("set BACKUPX_SQLSERVER_INTEGRATION=1 on a disposable SQL Server test instance")
	}
	password := os.Getenv("BACKUPX_SQLSERVER_TEST_PASSWORD")
	user := os.Getenv("BACKUPX_SQLSERVER_TEST_USER")
	if user == "" || password == "" {
		t.Fatal("BACKUPX_SQLSERVER_TEST_USER and BACKUPX_SQLSERVER_TEST_PASSWORD are required")
	}
	port := 1433
	if value := os.Getenv("BACKUPX_SQLSERVER_TEST_PORT"); value != "" {
		var err error
		port, err = strconv.Atoi(value)
		if err != nil {
			t.Fatal("invalid BACKUPX_SQLSERVER_TEST_PORT")
		}
	}
	opts := SQLServerOptions{
		InstanceName:           os.Getenv("BACKUPX_SQLSERVER_TEST_INSTANCE"),
		TrustServerCertificate: os.Getenv("BACKUPX_SQLSERVER_TEST_TRUST_CERTIFICATE") == "1",
	}
	extra, err := json.Marshal(opts)
	if err != nil {
		t.Fatal(err)
	}
	name := "backupx_vdi_it_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	task := TaskSpec{Name: name, Type: "sqlserver", TempDir: t.TempDir(), Database: DatabaseSpec{
		Host: "localhost", Port: port, User: user, Password: password, Names: []string{name}, ExtraConfig: string(extra),
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	runner := NewSQLServerRunner()
	execSQL := func(ctx context.Context, query string) error {
		return executeSQLServer(ctx, task.Database, opts, query, "")
	}
	if err := execSQL(ctx, "CREATE DATABASE ["+name+"]"); err != nil {
		t.Fatal(err)
	}
	// Cleanup only the database successfully created by this test; never a supplied name.
	defer func() {
		cleanupCtx, stop := context.WithTimeout(context.Background(), time.Minute)
		defer stop()
		if err := execSQL(cleanupCtx, "IF DB_ID(N'"+name+"') IS NOT NULL DROP DATABASE ["+name+"]"); err != nil {
			t.Errorf("cleanup disposable database %s: %v", name, err)
		}
	}()
	if err := execSQL(ctx, "CREATE TABLE ["+name+"].dbo.vdi_probe (id int PRIMARY KEY); INSERT INTO ["+name+"].dbo.vdi_probe VALUES (153)"); err != nil {
		t.Fatal(err)
	}
	result, err := runner.Run(ctx, task, NopLogWriter{})
	if err != nil {
		t.Fatal(err)
	}
	if err := execSQL(ctx, "DROP DATABASE ["+name+"]"); err != nil {
		t.Fatal(err)
	}
	if err := runner.Restore(ctx, task, result.ArtifactPath, NopLogWriter{}); err != nil {
		t.Fatal(err)
	}
	if err := execSQL(ctx, "IF (SELECT COUNT(*) FROM ["+name+"].dbo.vdi_probe WHERE id = 153) <> 1 THROW 50001, 'VDI restore data mismatch', 1; DBCC CHECKDB (["+name+"]) WITH NO_INFOMSGS"); err != nil {
		t.Fatal(err)
	}
}
