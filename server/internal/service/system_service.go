package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"backupx/server/internal/config"
)

type SystemInfo struct {
	Version       string `json:"version"`
	Mode          string `json:"mode"`
	StartedAt     string `json:"startedAt"`
	UptimeSeconds int64  `json:"uptimeSeconds"`
	DatabasePath  string `json:"databasePath"`
	DiskTotal     int64  `json:"diskTotal"`
	DiskFree      int64  `json:"diskFree"`
	DiskUsed      int64  `json:"diskUsed"`
}

type SystemService struct {
	cfg       config.Config
	version   string
	startedAt time.Time
}

func NewSystemService(cfg config.Config, version string, startedAt time.Time) *SystemService {
	return &SystemService{cfg: cfg, version: version, startedAt: startedAt}
}

// UpdateCheckResult 描述版本更新检查结果。
type UpdateCheckResult struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	HasUpdate      bool   `json:"hasUpdate"`
	ReleaseURL     string `json:"releaseUrl,omitempty"`
	ReleaseNotes   string `json:"releaseNotes,omitempty"`
	PublishedAt    string `json:"publishedAt,omitempty"`
	DownloadURL    string `json:"downloadUrl,omitempty"`
	DockerImage    string `json:"dockerImage,omitempty"`
}

const githubRepoAPI = "https://api.github.com/repos/Awuqing/BackupX/releases/latest"

// CheckUpdate 从 GitHub Releases 检查是否有新版本。
func (s *SystemService) CheckUpdate(ctx context.Context) (*UpdateCheckResult, error) {
	result := &UpdateCheckResult{
		CurrentVersion: s.version,
		DockerImage:    "awuqing/backupx",
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubRepoAPI, nil)
	if err != nil {
		return result, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "BackupX/"+s.version)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return result, fmt.Errorf("github api returned %d", resp.StatusCode)
	}

	var release struct {
		TagName    string `json:"tag_name"`
		HTMLURL    string `json:"html_url"`
		Body       string `json:"body"`
		Published  string `json:"published_at"`
		Assets     []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return result, fmt.Errorf("decode release: %w", err)
	}

	result.LatestVersion = release.TagName
	result.ReleaseURL = release.HTMLURL
	result.ReleaseNotes = release.Body
	result.PublishedAt = release.Published

	// 比较版本号（去 v 前缀后字符串比较）
	current := strings.TrimPrefix(s.version, "v")
	latest := strings.TrimPrefix(release.TagName, "v")
	result.HasUpdate = latest > current && current != "dev"

	// 匹配当前平台的下载链接
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	suffix := fmt.Sprintf("%s-%s.tar.gz", goos, goarch)
	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, suffix) {
			result.DownloadURL = asset.BrowserDownloadURL
			break
		}
	}

	return result, nil
}

func (s *SystemService) GetInfo(_ context.Context) *SystemInfo {
	now := time.Now().UTC()
	info := &SystemInfo{
		Version:       s.version,
		Mode:          s.cfg.Server.Mode,
		StartedAt:     s.startedAt.Format(time.RFC3339),
		UptimeSeconds: int64(now.Sub(s.startedAt).Seconds()),
		DatabasePath:  s.cfg.Database.Path,
	}
	dir := filepath.Dir(s.cfg.Database.Path)
	if dir == "" {
		dir = "."
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err == nil {
		info.DiskTotal = int64(stat.Blocks) * int64(stat.Bsize)
		info.DiskFree = int64(stat.Bavail) * int64(stat.Bsize)
		info.DiskUsed = info.DiskTotal - info.DiskFree
	}
	return info
}

// UpdateApplyResult 描述自动更新执行结果。
type UpdateApplyResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Output  string `json:"output,omitempty"`
}

// IsDockerEnvironment 检测当前是否运行在 Docker 容器中。
func (s *SystemService) IsDockerEnvironment() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	return false
}

// ApplyDockerUpdate 执行 Docker 自动更新：pull 新镜像 + recreate 容器。
// 容器会在 docker compose up -d 后自动重启为新版本。
func (s *SystemService) ApplyDockerUpdate(_ context.Context, targetVersion string) *UpdateApplyResult {
	if !s.IsDockerEnvironment() {
		return &UpdateApplyResult{Success: false, Message: "当前非 Docker 环境，请手动下载二进制更新"}
	}

	image := "awuqing/backupx"
	tag := strings.TrimSpace(targetVersion)
	if tag == "" {
		tag = "latest"
	}
	pullTarget := image + ":" + tag

	// Step 1: docker pull
	pullCmd := exec.Command("docker", "pull", pullTarget)
	pullOut, pullErr := pullCmd.CombinedOutput()
	if pullErr != nil {
		return &UpdateApplyResult{Success: false, Message: fmt.Sprintf("docker pull 失败: %v", pullErr), Output: string(pullOut)}
	}

	// Step 2: docker compose up -d（后台执行，容器会自重启）
	// 检测 compose 命令
	composeBin := "docker"
	composeArgs := []string{"compose", "up", "-d"}
	if _, err := exec.LookPath("docker-compose"); err == nil {
		composeBin = "docker-compose"
		composeArgs = []string{"up", "-d"}
	}

	// 异步执行，给 API 响应留时间
	go func() {
		time.Sleep(1 * time.Second)
		cmd := exec.Command(composeBin, composeArgs...)
		cmd.Dir = "/app" // Docker 容器中的工作目录
		_ = cmd.Run()
	}()

	return &UpdateApplyResult{
		Success: true,
		Message: fmt.Sprintf("已拉取 %s，容器即将自动重启到新版本", pullTarget),
		Output:  string(pullOut),
	}
}
