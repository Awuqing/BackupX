import type { StorageTargetFieldConfig, StorageTargetType } from '../../types/storage-targets'

const FIELD_CONFIG_MAP: Record<StorageTargetType, StorageTargetFieldConfig[]> = {
  local_disk: [
    {
      key: 'basePath',
      label: '基础目录',
      type: 'input',
      required: true,
      placeholder: '/data/backups',
      description: 'BackupX 将在该目录下创建和管理备份文件。',
    },
  ],
  s3: [
    {
      key: 'endpoint',
      label: 'Endpoint',
      type: 'input',
      required: true,
      placeholder: 'https://s3.amazonaws.com',
    },
    {
      key: 'region',
      label: '区域',
      type: 'input',
      required: true,
      placeholder: 'ap-east-1',
    },
    {
      key: 'bucket',
      label: 'Bucket',
      type: 'input',
      required: true,
      placeholder: 'backupx-prod',
    },
    {
      key: 'accessKeyId',
      label: 'Access Key ID',
      type: 'input',
      required: true,
      sensitive: true,
      placeholder: 'AKIA...',
    },
    {
      key: 'secretAccessKey',
      label: 'Secret Access Key',
      type: 'password',
      required: true,
      sensitive: true,
      placeholder: '输入新的 Secret Access Key',
    },
    {
      key: 'forcePathStyle',
      label: '强制 Path Style',
      type: 'switch',
      description: 'MinIO 或部分兼容对象存储通常需要开启。',
    },
  ],
  webdav: [
    {
      key: 'endpoint',
      label: 'WebDAV 地址',
      type: 'input',
      required: true,
      placeholder: 'https://dav.example.com/remote.php/dav/files/admin',
    },
    {
      key: 'username',
      label: '用户名',
      type: 'input',
      required: true,
      placeholder: 'admin',
    },
    {
      key: 'password',
      label: '密码',
      type: 'password',
      required: true,
      sensitive: true,
      placeholder: '输入新的 WebDAV 密码',
    },
    {
      key: 'basePath',
      label: '基础目录',
      type: 'input',
      placeholder: '/backupx',
    },
  ],
  google_drive: [
    {
      key: 'clientId',
      label: 'Client ID',
      type: 'input',
      required: true,
      sensitive: true,
      placeholder: 'Google OAuth Client ID',
    },
    {
      key: 'clientSecret',
      label: 'Client Secret',
      type: 'password',
      required: true,
      sensitive: true,
      placeholder: '输入新的 Google Client Secret',
    },
    {
      key: 'folderId',
      label: '目标文件夹 ID',
      type: 'input',
      placeholder: '留空则使用根目录',
    },
  ],
  aliyun_oss: [
    {
      key: 'region',
      label: '区域 (Region)',
      type: 'input',
      required: true,
      placeholder: 'cn-hangzhou',
      description: '如 cn-hangzhou, cn-shanghai, cn-beijing, cn-shenzhen 等。系统会自动组装 Endpoint。',
    },
    {
      key: 'bucket',
      label: 'Bucket',
      type: 'input',
      required: true,
      placeholder: 'my-backup-bucket',
    },
    {
      key: 'accessKeyId',
      label: 'AccessKey ID',
      type: 'input',
      required: true,
      sensitive: true,
      placeholder: 'LTAI...',
    },
    {
      key: 'secretAccessKey',
      label: 'AccessKey Secret',
      type: 'password',
      required: true,
      sensitive: true,
      placeholder: '输入新的 AccessKey Secret',
    },
    {
      key: 'internalNetwork',
      label: '使用内网 Endpoint',
      type: 'switch',
      description: '同一区域的 ECS 实例可启用内网传输，节省流量费用。',
    },
  ],
  tencent_cos: [
    {
      key: 'region',
      label: '区域 (Region)',
      type: 'input',
      required: true,
      placeholder: 'ap-guangzhou',
      description: '如 ap-guangzhou, ap-shanghai, ap-beijing, ap-chengdu 等。系统会自动组装 Endpoint。',
    },
    {
      key: 'bucket',
      label: 'Bucket',
      type: 'input',
      required: true,
      placeholder: 'backup-1250000000',
      description: '格式为 BucketName-APPID，如 backup-1250000000。',
    },
    {
      key: 'accessKeyId',
      label: 'SecretId',
      type: 'input',
      required: true,
      sensitive: true,
      placeholder: 'AKIDxxxxxxxx',
    },
    {
      key: 'secretAccessKey',
      label: 'SecretKey',
      type: 'password',
      required: true,
      sensitive: true,
      placeholder: '输入新的 SecretKey',
    },
  ],
  qiniu_kodo: [
    {
      key: 'region',
      label: '区域 (Region)',
      type: 'input',
      required: true,
      placeholder: 'z0',
      description: '支持 z0(华东), cn-east-2(华东-浙江2), z1(华北), z2(华南), na0(北美), as0(东南亚)。',
    },
    {
      key: 'bucket',
      label: 'Bucket',
      type: 'input',
      required: true,
      placeholder: 'my-backup',
    },
    {
      key: 'accessKeyId',
      label: 'AccessKey',
      type: 'input',
      required: true,
      sensitive: true,
      placeholder: '七牛云 AccessKey',
    },
    {
      key: 'secretAccessKey',
      label: 'SecretKey',
      type: 'password',
      required: true,
      sensitive: true,
      placeholder: '输入新的 SecretKey',
    },
  ],
  rclone: [], // 动态表单，字段从 API 获取（见 StorageTargetFormDrawer）
  ftp: [
    {
      key: 'host',
      label: '主机地址',
      type: 'input',
      required: true,
      placeholder: 'ftp.example.com',
    },
    {
      key: 'port',
      label: '端口',
      type: 'input',
      placeholder: '21',
      description: '默认 FTP 端口为 21。',
    },
    {
      key: 'username',
      label: '用户名',
      type: 'input',
      required: true,
      placeholder: 'backup_user',
    },
    {
      key: 'password',
      label: '密码',
      type: 'password',
      required: true,
      sensitive: true,
      placeholder: '输入新的 FTP 密码',
    },
    {
      key: 'basePath',
      label: '基础目录',
      type: 'input',
      placeholder: '/backups',
      description: 'FTP 服务器上的目标目录，留空使用根目录。',
    },
    {
      key: 'useTLS',
      label: '使用 TLS (FTPS)',
      type: 'switch',
      description: '启用 Explicit TLS 加密连接。',
    },
  ],
}

export function getStorageTargetFieldConfigs(type: StorageTargetType) {
  return FIELD_CONFIG_MAP[type]
}

export function getStorageTargetTypeLabel(type: StorageTargetType) {
  switch (type) {
    case 'local_disk':
      return '本地磁盘'
    case 'google_drive':
      return 'Google Drive'
    case 's3':
      return 'S3 Compatible'
    case 'webdav':
      return 'WebDAV'
    case 'aliyun_oss':
      return '阿里云 OSS'
    case 'tencent_cos':
      return '腾讯云 COS'
    case 'qiniu_kodo':
      return '七牛云 Kodo'
    case 'ftp':
      return 'FTP'
    case 'rclone':
      return 'Rclone (70+ 后端)'
    default:
      return type
  }
}

export const storageTargetTypeOptions = [
  { label: '本地磁盘', value: 'local_disk' },
  { label: '阿里云 OSS', value: 'aliyun_oss' },
  { label: '腾讯云 COS', value: 'tencent_cos' },
  { label: '七牛云 Kodo', value: 'qiniu_kodo' },
  { label: 'S3 Compatible', value: 's3' },
  { label: 'Google Drive', value: 'google_drive' },
  { label: 'WebDAV', value: 'webdav' },
  { label: 'FTP', value: 'ftp' },
  { label: 'Rclone (70+ 后端)', value: 'rclone' },
] as const
