package googledrive

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"backupx/server/internal/storage"
	"golang.org/x/oauth2"
	googleoauth "golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)


type fileInfo struct {
	ID           string
	Name         string
	Size         int64
	ModifiedTime time.Time
}

type client interface {
	TestConnection(context.Context, string) error
	Upload(context.Context, string, string, io.Reader) error
	Download(context.Context, string, string) (io.ReadCloser, error)
	Delete(context.Context, string, string) error
	List(context.Context, string, string) ([]storage.ObjectInfo, error)
	EnsureFolder(ctx context.Context, parentID, name string) (string, error)
}

type Provider struct {
	client      client
	rootFolder  string // user-configured folderId, empty means Drive root
	folderCache map[string]string // cache: path -> folderID
}

type Factory struct {
	newClient func(context.Context, storage.GoogleDriveConfig) (client, error)
}

func NewFactory() Factory {
	return Factory{newClient: newDriveClient}
}

func (Factory) Type() storage.ProviderType { return storage.ProviderTypeGoogleDrive }
func (Factory) SensitiveFields() []string {
	return []string{"clientId", "clientSecret", "refreshToken"}
}

func (f Factory) New(ctx context.Context, rawConfig map[string]any) (storage.StorageProvider, error) {
	cfg, err := storage.DecodeConfig[storage.GoogleDriveConfig](rawConfig)
	if err != nil {
		return nil, err
	}
	cfg = cfg.Normalize()
	if strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecret) == "" {
		return nil, fmt.Errorf("google drive client credentials are required")
	}
	if strings.TrimSpace(cfg.RefreshToken) == "" {
		return nil, fmt.Errorf("google drive refresh token is required")
	}
	newClient := f.newClient
	if newClient == nil {
		newClient = NewFactory().newClient
	}
	client, err := newClient(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Provider{
		client:      client,
		rootFolder:  strings.TrimSpace(cfg.FolderID),
		folderCache: make(map[string]string),
	}, nil
}

func (p *Provider) Type() storage.ProviderType { return storage.ProviderTypeGoogleDrive }

// ensureFolderPath creates nested folders for a path like "BackupX/file/260308"
// and returns the deepest folder's ID.
func (p *Provider) ensureFolderPath(ctx context.Context, folderPath string) (string, error) {
	if folderPath == "" || folderPath == "." {
		return p.rootFolder, nil
	}
	if cached, ok := p.folderCache[folderPath]; ok {
		return cached, nil
	}
	parts := strings.Split(path.Clean(folderPath), "/")
	parentID := p.rootFolder
	builtPath := ""
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if builtPath == "" {
			builtPath = part
		} else {
			builtPath = builtPath + "/" + part
		}
		if cached, ok := p.folderCache[builtPath]; ok {
			parentID = cached
			continue
		}
		folderID, err := p.client.EnsureFolder(ctx, parentID, part)
		if err != nil {
			return "", fmt.Errorf("ensure folder %s: %w", builtPath, err)
		}
		p.folderCache[builtPath] = folderID
		parentID = folderID
	}
	return parentID, nil
}

func (p *Provider) TestConnection(ctx context.Context) error {
	return p.client.TestConnection(ctx, p.rootFolder)
}

func (p *Provider) Upload(ctx context.Context, objectKey string, reader io.Reader, _ int64, _ map[string]string) error {
	dir := path.Dir(objectKey)
	folderID, err := p.ensureFolderPath(ctx, dir)
	if err != nil {
		return err
	}
	return p.client.Upload(ctx, folderID, objectKey, reader)
}

func (p *Provider) Download(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	dir := path.Dir(objectKey)
	folderID, err := p.ensureFolderPath(ctx, dir)
	if err != nil {
		return nil, err
	}
	return p.client.Download(ctx, folderID, objectKey)
}

func (p *Provider) Delete(ctx context.Context, objectKey string) error {
	dir := path.Dir(objectKey)
	folderID, err := p.ensureFolderPath(ctx, dir)
	if err != nil {
		return err
	}
	return p.client.Delete(ctx, folderID, objectKey)
}

func (p *Provider) List(ctx context.Context, prefix string) ([]storage.ObjectInfo, error) {
	dir := path.Dir(prefix)
	folderID, err := p.ensureFolderPath(ctx, dir)
	if err != nil {
		return nil, err
	}
	return p.client.List(ctx, folderID, prefix)
}

type driveClient struct {
	service *drive.Service
}

func newDriveClient(ctx context.Context, cfg storage.GoogleDriveConfig) (client, error) {
	cfg = cfg.Normalize()
	oauthCfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     googleoauth.Endpoint,
		Scopes:       []string{drive.DriveScope},
	}
	httpClient := oauthCfg.Client(ctx, &oauth2.Token{RefreshToken: cfg.RefreshToken})
	service, err := drive.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("create google drive service: %w", err)
	}
	return &driveClient{service: service}, nil
}

func (c *driveClient) TestConnection(ctx context.Context, folderID string) error {
	if strings.TrimSpace(folderID) == "" {
		_, err := c.service.About.Get().Fields("user").Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("test google drive connection: %w", err)
		}
		return nil
	}
	_, err := c.service.Files.Get(folderID).Fields("id").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("test google drive folder: %w", err)
	}
	return nil
}

func (c *driveClient) EnsureFolder(ctx context.Context, parentID, name string) (string, error) {
	// Search for existing folder
	query := fmt.Sprintf("name = '%s' and mimeType = 'application/vnd.google-apps.folder' and trashed = false", escapeQuery(name))
	if strings.TrimSpace(parentID) != "" {
		query += fmt.Sprintf(" and '%s' in parents", escapeQuery(parentID))
	} else {
		query += " and 'root' in parents"
	}
	result, err := c.service.Files.List().Q(query).PageSize(1).Fields("files(id)").Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("search for folder %s: %w", name, err)
	}
	if len(result.Files) > 0 {
		return result.Files[0].Id, nil
	}
	// Create the folder
	folder := &drive.File{
		Name:     name,
		MimeType: "application/vnd.google-apps.folder",
	}
	if strings.TrimSpace(parentID) != "" {
		folder.Parents = []string{parentID}
	}
	created, err := c.service.Files.Create(folder).Fields("id").Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("create folder %s: %w", name, err)
	}
	return created.Id, nil
}

func (c *driveClient) Upload(ctx context.Context, folderID, objectKey string, reader io.Reader) error {
	file := &drive.File{Name: path.Base(objectKey)}
	if strings.TrimSpace(folderID) != "" {
		file.Parents = []string{folderID}
	}
	_, err := c.service.Files.Create(file).Media(reader).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("upload google drive object: %w", err)
	}
	return nil
}

func (c *driveClient) Download(ctx context.Context, folderID, objectKey string) (io.ReadCloser, error) {
	file, err := c.findFile(ctx, folderID, objectKey)
	if err != nil {
		return nil, err
	}
	response, err := c.service.Files.Get(file.ID).Context(ctx).Download()
	if err != nil {
		return nil, fmt.Errorf("download google drive object: %w", err)
	}
	return response.Body, nil
}

func (c *driveClient) Delete(ctx context.Context, folderID, objectKey string) error {
	file, err := c.findFile(ctx, folderID, objectKey)
	if err != nil {
		return err
	}
	if err := c.service.Files.Delete(file.ID).Context(ctx).Do(); err != nil {
		return fmt.Errorf("delete google drive object: %w", err)
	}
	return nil
}

func (c *driveClient) List(ctx context.Context, folderID, prefix string) ([]storage.ObjectInfo, error) {
	query := "trashed = false"
	if strings.TrimSpace(folderID) != "" {
		query += fmt.Sprintf(" and '%s' in parents", escapeQuery(folderID))
	}
	if strings.TrimSpace(prefix) != "" {
		query += fmt.Sprintf(" and name contains '%s'", escapeQuery(prefix))
	}
	result, err := c.service.Files.List().Q(query).Fields("files(id,name,size,modifiedTime)").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("list google drive objects: %w", err)
	}
	items := make([]storage.ObjectInfo, 0, len(result.Files))
	for _, file := range result.Files {
		modifiedAt, _ := time.Parse(time.RFC3339, file.ModifiedTime)
		items = append(items, storage.ObjectInfo{Key: file.Name, Size: file.Size, UpdatedAt: modifiedAt.UTC()})
	}
	return items, nil
}

func (c *driveClient) findFile(ctx context.Context, folderID, objectKey string) (*fileInfo, error) {
	query := fmt.Sprintf("name = '%s' and trashed = false", escapeQuery(path.Base(objectKey)))
	if strings.TrimSpace(folderID) != "" {
		query += fmt.Sprintf(" and '%s' in parents", escapeQuery(folderID))
	}
	result, err := c.service.Files.List().Q(query).PageSize(1).Fields("files(id,name,size,modifiedTime)").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("query google drive object: %w", err)
	}
	if len(result.Files) == 0 {
		return nil, fmt.Errorf("google drive object not found: %s", objectKey)
	}
	file := result.Files[0]
	modifiedAt, _ := time.Parse(time.RFC3339, file.ModifiedTime)
	return &fileInfo{ID: file.Id, Name: file.Name, Size: file.Size, ModifiedTime: modifiedAt.UTC()}, nil
}

func escapeQuery(value string) string {
	return strings.ReplaceAll(value, "'", "\\'")
}

