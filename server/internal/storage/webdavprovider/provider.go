package webdavprovider

import "backupx/server/internal/storage/webdav"

type Factory = webdav.Factory

func NewFactory() Factory {
	return webdav.NewFactory()
}
