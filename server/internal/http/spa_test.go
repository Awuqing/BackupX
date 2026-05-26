package http

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestResolveWebRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("explicit configured dir with index.html", func(t *testing.T) {
		dir := t.TempDir()
		writeIndex(t, dir)
		got := resolveWebRoot(dir)
		abs, _ := filepath.Abs(dir)
		if got != abs {
			t.Fatalf("resolveWebRoot(%q) = %q, want %q", dir, got, abs)
		}
	})

	t.Run("configured dir without index.html falls through to none", func(t *testing.T) {
		dir := t.TempDir() // no index.html, and no conventional ./web in CWD during test
		if got := resolveWebRoot(dir); got != "" {
			// 允许 CWD 恰好存在约定目录的环境，但临时目录本身不应被选中。
			abs, _ := filepath.Abs(dir)
			if got == abs {
				t.Fatalf("expected dir without index.html to be skipped, got %q", got)
			}
		}
	})

	t.Run("empty configured uses auto-detect order", func(t *testing.T) {
		// 切到一个仅含 ./web/dist/index.html 的临时工作目录，验证自动探测。
		root := t.TempDir()
		distDir := filepath.Join(root, "web", "dist")
		if err := os.MkdirAll(distDir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeIndex(t, distDir)

		restore := chdir(t, root)
		defer restore()

		// 以 chdir 之后的实际工作目录为基准计算期望值，避免 macOS 上
		// /var → /private/var 符号链接导致字符串不一致。
		wd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(wd, "web", "dist")

		got := resolveWebRoot("")
		if got != want {
			t.Fatalf("auto-detect = %q, want %q", got, want)
		}
	})
}

func TestIsReservedBackendPath(t *testing.T) {
	reserved := []string{"/health", "/ready", "/metrics", "/api", "/install", "/api/", "/api/system/info", "/install/abc", "/install/abc/compose.yml"}
	for _, p := range reserved {
		if !isReservedBackendPath(p) {
			t.Errorf("isReservedBackendPath(%q) = false, want true", p)
		}
	}
	notReserved := []string{"/", "/dashboard", "/assets/app.js", "/installer", "/apidocs", "/favicon.ico"}
	for _, p := range notReserved {
		if isReservedBackendPath(p) {
			t.Errorf("isReservedBackendPath(%q) = true, want false", p)
		}
	}
}

func TestSpaFileServer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	webRoot := t.TempDir()
	writeIndexContent(t, webRoot, "<!doctype html><title>BackupX</title>")
	assetsDir := filepath.Join(webRoot, "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "app.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}

	apiNotFoundHit := false
	apiNotFound := func(c *gin.Context) {
		apiNotFoundHit = true
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND"})
	}

	engine := gin.New()
	engine.NoRoute(spaFileServer(webRoot, apiNotFound))

	cases := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string // 子串；为空表示不校验
		wantAPI404 bool
	}{
		{name: "root serves index", method: http.MethodGet, path: "/", wantStatus: 200, wantBody: "BackupX"},
		{name: "spa route falls back to index", method: http.MethodGet, path: "/dashboard", wantStatus: 200, wantBody: "BackupX"},
		{name: "real asset served", method: http.MethodGet, path: "/assets/app.js", wantStatus: 200, wantBody: "console.log"},
		{name: "api path returns json 404", method: http.MethodGet, path: "/api/garbage", wantStatus: 404, wantAPI404: true},
		{name: "health returns json 404 via reserved", method: http.MethodGet, path: "/health", wantStatus: 404, wantAPI404: true},
		{name: "non-GET on spa path is api 404", method: http.MethodPost, path: "/dashboard", wantStatus: 404, wantAPI404: true},
		{name: "directory falls back to index", method: http.MethodGet, path: "/assets/", wantStatus: 200, wantBody: "BackupX"},
		// 含 ".." 的请求路径被 net/http 在文件服务层直接拒绝（400 invalid URL path），
		// 绝不会泄露 webRoot 之外的文件；这是在 filepath.Rel 校验之上的纵深防御。
		{name: "traversal rejected, never serves passwd", method: http.MethodGet, path: "/../../etc/passwd", wantStatus: 400, wantBody: "invalid URL path"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			apiNotFoundHit = false
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body=%q)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantBody != "" && !contains(rec.Body.String(), tc.wantBody) {
				t.Fatalf("body %q does not contain %q", rec.Body.String(), tc.wantBody)
			}
			if tc.wantAPI404 != apiNotFoundHit {
				t.Fatalf("apiNotFoundHit = %v, want %v", apiNotFoundHit, tc.wantAPI404)
			}
		})
	}
}

func writeIndex(t *testing.T, dir string) {
	t.Helper()
	writeIndexContent(t, dir, "<!doctype html><title>BackupX</title>")
}

func writeIndexContent(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func chdir(t *testing.T, dir string) func() {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() { _ = os.Chdir(orig) }
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
