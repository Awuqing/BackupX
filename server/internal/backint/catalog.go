package backint

import (
	"fmt"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
)

// CatalogEntry 是 Backint 目录条目，建立 BID (备份 ID) 与对象键的映射。
//
// BID 是 Backint Agent 返回给 SAP HANA 的唯一标识，HANA 后续用它作为 RESTORE/DELETE
// 的句柄。Agent 用 catalog 查询该 BID 对应的实际存储对象键。
type CatalogEntry struct {
	ID         uint      `gorm:"primaryKey"`
	EBID       string    `gorm:"column:ebid;uniqueIndex;size:128;not null"`
	ObjectKey  string    `gorm:"column:object_key;size:512;not null"`
	SourcePath string    `gorm:"column:source_path;size:1024"`
	Size       int64     `gorm:"column:size"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

// TableName 指定表名，避免 GORM 自动复数化。
func (CatalogEntry) TableName() string { return "backint_catalog" }

// Catalog 是本地 Backint 目录（SQLite 后端）。
type Catalog struct {
	db *gorm.DB
}

// OpenCatalog 打开或创建 catalog 数据库。
func OpenCatalog(dbPath string) (*Catalog, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open catalog: %w", err)
	}
	if err := db.AutoMigrate(&CatalogEntry{}); err != nil {
		return nil, fmt.Errorf("migrate catalog: %w", err)
	}
	return &Catalog{db: db}, nil
}

// Close 关闭底层连接。
func (c *Catalog) Close() error {
	if c.db == nil {
		return nil
	}
	sqlDB, err := c.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Put 插入或更新一条记录。
func (c *Catalog) Put(entry CatalogEntry) error {
	if entry.EBID == "" {
		return fmt.Errorf("ebid is required")
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	// Upsert：EBID 冲突时更新 object_key/size/source_path
	return c.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "ebid"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"object_key", "source_path", "size", "created_at",
		}),
	}).Create(&entry).Error
}

// Get 通过 EBID 查询条目。未找到返回 (nil, nil)。
func (c *Catalog) Get(ebid string) (*CatalogEntry, error) {
	var entry CatalogEntry
	err := c.db.Where("ebid = ?", ebid).First(&entry).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// Delete 删除一条记录。
func (c *Catalog) Delete(ebid string) error {
	return c.db.Where("ebid = ?", ebid).Delete(&CatalogEntry{}).Error
}

// List 列出全部条目。
func (c *Catalog) List() ([]CatalogEntry, error) {
	var entries []CatalogEntry
	if err := c.db.Order("created_at DESC").Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}
