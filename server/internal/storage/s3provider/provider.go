package s3provider

import "backupx/server/internal/storage/s3"

type Factory = s3.Factory

func NewFactory() Factory {
	return s3.NewFactory()
}
