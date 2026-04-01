import type { StorageTargetFieldConfig, StorageTargetType } from '../../types/storage-targets'

// 内置类型的静态字段配置（定制化配置结构）
const BUILTIN_FIELD_CONFIG: Record<string, StorageTargetFieldConfig[]> = {
  local_disk: [
    { key: 'basePath', label: '基础目录', type: 'input', required: true, placeholder: '/data/backups', description: 'BackupX 将在该目录下创建和管理备份文件。' },
  ],
  s3: [
    { key: 'endpoint', label: 'Endpoint', type: 'input', required: true, placeholder: 'https://s3.amazonaws.com' },
    { key: 'region', label: '区域', type: 'input', required: true, placeholder: 'ap-east-1' },
    { key: 'bucket', label: 'Bucket', type: 'input', required: true, placeholder: 'backupx-prod' },
    { key: 'accessKeyId', label: 'Access Key ID', type: 'input', required: true, sensitive: true, placeholder: 'AKIA...' },
    { key: 'secretAccessKey', label: 'Secret Access Key', type: 'password', required: true, sensitive: true },
    { key: 'forcePathStyle', label: '强制 Path Style', type: 'switch', description: 'MinIO 等兼容存储需要开启。' },
  ],
  webdav: [
    { key: 'endpoint', label: 'WebDAV 地址', type: 'input', required: true, placeholder: 'https://dav.example.com/...' },
    { key: 'username', label: '用户名', type: 'input', required: true },
    { key: 'password', label: '密码', type: 'password', required: true, sensitive: true },
    { key: 'basePath', label: '基础目录', type: 'input', placeholder: '/backupx' },
  ],
  google_drive: [
    { key: 'clientId', label: 'Client ID', type: 'input', required: true, sensitive: true },
    { key: 'clientSecret', label: 'Client Secret', type: 'password', required: true, sensitive: true },
    { key: 'folderId', label: '目标文件夹 ID', type: 'input', placeholder: '留空则使用根目录' },
  ],
  aliyun_oss: [
    { key: 'region', label: '区域', type: 'input', required: true, placeholder: 'cn-hangzhou' },
    { key: 'bucket', label: 'Bucket', type: 'input', required: true },
    { key: 'accessKeyId', label: 'AccessKey ID', type: 'input', required: true, sensitive: true },
    { key: 'secretAccessKey', label: 'AccessKey Secret', type: 'password', required: true, sensitive: true },
    { key: 'internalNetwork', label: '使用内网', type: 'switch' },
  ],
  tencent_cos: [
    { key: 'region', label: '区域', type: 'input', required: true, placeholder: 'ap-guangzhou' },
    { key: 'bucket', label: 'Bucket', type: 'input', required: true, placeholder: 'backup-1250000000' },
    { key: 'accessKeyId', label: 'SecretId', type: 'input', required: true, sensitive: true },
    { key: 'secretAccessKey', label: 'SecretKey', type: 'password', required: true, sensitive: true },
  ],
  qiniu_kodo: [
    { key: 'region', label: '区域', type: 'input', required: true, placeholder: 'z0' },
    { key: 'bucket', label: 'Bucket', type: 'input', required: true },
    { key: 'accessKeyId', label: 'AccessKey', type: 'input', required: true, sensitive: true },
    { key: 'secretAccessKey', label: 'SecretKey', type: 'password', required: true, sensitive: true },
  ],
  ftp: [
    { key: 'host', label: '主机地址', type: 'input', required: true, placeholder: 'ftp.example.com' },
    { key: 'port', label: '端口', type: 'input', placeholder: '21' },
    { key: 'username', label: '用户名', type: 'input', required: true },
    { key: 'password', label: '密码', type: 'password', required: true, sensitive: true },
    { key: 'basePath', label: '基础目录', type: 'input', placeholder: '/backups' },
    { key: 'useTLS', label: 'TLS (FTPS)', type: 'switch' },
  ],
}

const BUILTIN_TYPES = new Set(Object.keys(BUILTIN_FIELD_CONFIG))

/** 是否为内置类型 */
export function isBuiltinType(type: StorageTargetType): boolean {
  return BUILTIN_TYPES.has(type)
}

/** 获取静态字段配置 */
export function getStorageTargetFieldConfigs(type: StorageTargetType): StorageTargetFieldConfig[] {
  return BUILTIN_FIELD_CONFIG[type] ?? []
}

const BUILTIN_LABELS: Record<string, string> = {
  local_disk: '本地磁盘', google_drive: 'Google Drive', s3: 'S3 Compatible',
  webdav: 'WebDAV', aliyun_oss: '阿里云 OSS', tencent_cos: '腾讯云 COS',
  qiniu_kodo: '七牛云 Kodo', ftp: 'FTP', rclone: 'Rclone',
}

export function getStorageTargetTypeLabel(type: StorageTargetType): string {
  return BUILTIN_LABELS[type] || type.toUpperCase()
}

/** 内置类型选项（下拉框"常用"分组） */
export const builtinTypeOptions = [
  { label: '本地磁盘', value: 'local_disk' },
  { label: '阿里云 OSS', value: 'aliyun_oss' },
  { label: '腾讯云 COS', value: 'tencent_cos' },
  { label: '七牛云 Kodo', value: 'qiniu_kodo' },
  { label: 'S3 Compatible', value: 's3' },
  { label: 'Google Drive', value: 'google_drive' },
  { label: 'WebDAV', value: 'webdav' },
  { label: 'FTP', value: 'ftp' },
]
