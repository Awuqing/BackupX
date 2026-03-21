package ftp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"backupx/server/internal/storage"

	"github.com/jlaffaye/ftp"
)

// Provider implements storage.StorageProvider for FTP.
type Provider struct {
	config storage.FTPConfig
}

// Factory creates FTP storage providers.
type Factory struct{}

// NewFactory returns a new FTP Factory.
func NewFactory() Factory {
	return Factory{}
}

func (Factory) Type() storage.ProviderType { return storage.ProviderTypeFTP }
func (Factory) SensitiveFields() []string  { return []string{"username", "password"} }

func (f Factory) New(_ context.Context, rawConfig map[string]any) (storage.StorageProvider, error) {
	cfg, err := storage.DecodeConfig[storage.FTPConfig](rawConfig)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return nil, fmt.Errorf("FTP host is required")
	}
	if cfg.Port == 0 {
		cfg.Port = 21
	}
	return &Provider{config: cfg}, nil
}

func (p *Provider) Type() storage.ProviderType { return storage.ProviderTypeFTP }

// dial establishes a connection to the FTP server and logs in.
func (p *Provider) dial() (*ftp.ServerConn, error) {
	addr := fmt.Sprintf("%s:%d", p.config.Host, p.config.Port)

	var opts []ftp.DialOption
	opts = append(opts, ftp.DialWithTimeout(30*time.Second))
	if p.config.UseTLS {
		opts = append(opts, ftp.DialWithExplicitTLS(nil))
	}

	conn, err := ftp.Dial(addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("connect to FTP server %s: %w", addr, err)
	}

	username := p.config.Username
	if username == "" {
		username = "anonymous"
	}
	if err := conn.Login(username, p.config.Password); err != nil {
		conn.Quit()
		return nil, fmt.Errorf("FTP login: %w", err)
	}

	return conn, nil
}

func (p *Provider) TestConnection(_ context.Context) error {
	conn, err := p.dial()
	if err != nil {
		return err
	}
	defer conn.Quit()

	basePath := p.normalizeBasePath()
	if err := p.ensureDir(conn, basePath); err != nil {
		return fmt.Errorf("ensure FTP base path: %w", err)
	}
	_, err = conn.List(basePath)
	if err != nil {
		return fmt.Errorf("list FTP base path: %w", err)
	}
	return nil
}

func (p *Provider) Upload(_ context.Context, objectKey string, reader io.Reader, _ int64, _ map[string]string) error {
	conn, err := p.dial()
	if err != nil {
		return err
	}
	defer conn.Quit()

	objectPath := p.resolvePath(objectKey)
	dir := path.Dir(objectPath)
	if err := p.ensureDir(conn, dir); err != nil {
		return fmt.Errorf("create FTP directories: %w", err)
	}

	// Read all data into buffer since FTP STOR needs the full stream
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read upload data: %w", err)
	}

	if err := conn.Stor(objectPath, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("FTP upload: %w", err)
	}
	return nil
}

func (p *Provider) Download(_ context.Context, objectKey string) (io.ReadCloser, error) {
	conn, err := p.dial()
	if err != nil {
		return nil, err
	}

	objectPath := p.resolvePath(objectKey)
	resp, err := conn.Retr(objectPath)
	if err != nil {
		conn.Quit()
		return nil, fmt.Errorf("FTP download: %w", err)
	}

	// Wrap the response to also close the FTP connection when done
	return &ftpReadCloser{ReadCloser: resp, conn: conn}, nil
}

func (p *Provider) Delete(_ context.Context, objectKey string) error {
	conn, err := p.dial()
	if err != nil {
		return err
	}
	defer conn.Quit()

	objectPath := p.resolvePath(objectKey)
	if err := conn.Delete(objectPath); err != nil {
		return fmt.Errorf("FTP delete: %w", err)
	}
	return nil
}

func (p *Provider) List(_ context.Context, prefix string) ([]storage.ObjectInfo, error) {
	conn, err := p.dial()
	if err != nil {
		return nil, err
	}
	defer conn.Quit()

	basePath := p.normalizeBasePath()
	entries, err := conn.List(basePath)
	if err != nil {
		return nil, fmt.Errorf("FTP list: %w", err)
	}

	items := make([]storage.ObjectInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.Type == ftp.EntryTypeFolder {
			continue
		}
		key := strings.TrimPrefix(path.Join(strings.TrimPrefix(basePath, "/"), entry.Name), "/")
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			continue
		}
		items = append(items, storage.ObjectInfo{
			Key:       key,
			Size:      int64(entry.Size),
			UpdatedAt: entry.Time.UTC(),
		})
	}
	return items, nil
}

// normalizeBasePath returns a cleaned base path with leading slash.
func (p *Provider) normalizeBasePath() string {
	clean := path.Clean("/" + strings.TrimSpace(p.config.BasePath))
	if clean == "." {
		return "/"
	}
	return clean
}

// resolvePath returns the full FTP path for the given object key.
func (p *Provider) resolvePath(objectKey string) string {
	cleanKey := path.Clean("/" + strings.TrimSpace(objectKey))
	return path.Clean(path.Join(p.normalizeBasePath(), cleanKey))
}

// ensureDir creates all directories in the path recursively.
func (p *Provider) ensureDir(conn *ftp.ServerConn, dirPath string) error {
	parts := strings.Split(strings.Trim(dirPath, "/"), "/")
	current := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		current = current + "/" + part
		if err := conn.MakeDir(current); err != nil {
			// Ignore errors if directory already exists
			// FTP doesn't have a standard "mkdir if not exists"
			_ = err
		}
	}
	return nil
}

// ftpReadCloser wraps an io.ReadCloser from FTP and closes the connection when done.
type ftpReadCloser struct {
	io.ReadCloser
	conn *ftp.ServerConn
}

func (f *ftpReadCloser) Close() error {
	err := f.ReadCloser.Close()
	if f.conn != nil {
		f.conn.Quit()
	}
	return err
}
