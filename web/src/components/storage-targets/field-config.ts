import type { StorageTargetFieldConfig, StorageTargetType } from '../../types/storage-targets'

// ---------------------------------------------------------------------------
// 内置类型的静态字段配置
// ---------------------------------------------------------------------------

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

export function isBuiltinType(type: StorageTargetType): boolean {
  return BUILTIN_TYPES.has(type)
}

export function getStorageTargetFieldConfigs(type: StorageTargetType): StorageTargetFieldConfig[] {
  return BUILTIN_FIELD_CONFIG[type] ?? []
}

// ---------------------------------------------------------------------------
// 存储类型完整列表（分类、中文标注、去重）
// ---------------------------------------------------------------------------

export interface TypeOption {
  label: string
  value: string
  group: string
}

// rclone 后端中不适合做存储目标的（工具类/代理类/只读类）
const EXCLUDED_BACKENDS = new Set([
  'alias', 'cache', 'http', 'archive', 'memory', 'tardigrade', // tardigrade = storj 别名
  'union', 'crypt', 'chunker', 'compress', 'hasher', 'combine',
  'local', // 用内置 local_disk 替代
  'drive', // 用内置 google_drive 替代（避免和 rclone 的 drive 重复）
])

// 内置类型（带中文标签的定制化类型，优先展示）
const BUILTIN_OPTIONS: TypeOption[] = [
  { label: '本地磁盘', value: 'local_disk', group: '常用' },
  { label: 'S3 兼容存储（AWS / MinIO / 阿里云 / 腾讯云等）', value: 's3', group: '常用' },
  { label: '阿里云 OSS', value: 'aliyun_oss', group: '常用' },
  { label: '腾讯云 COS', value: 'tencent_cos', group: '常用' },
  { label: '七牛云 Kodo', value: 'qiniu_kodo', group: '常用' },
  { label: 'Google Drive', value: 'google_drive', group: '常用' },
  { label: 'WebDAV（Nextcloud / 坚果云等）', value: 'webdav', group: '常用' },
  { label: 'FTP / FTPS', value: 'ftp', group: '常用' },
]

// rclone 后端的中文标注（仅标注常见的，其余用原始描述）
const RCLONE_LABELS: Record<string, { label: string; group: string }> = {
  sftp:                { label: 'SFTP（SSH 文件传输）', group: '文件传输' },
  smb:                 { label: 'SMB / CIFS（Windows 共享）', group: '文件传输' },
  azureblob:           { label: 'Azure Blob 存储', group: '云存储' },
  azurefiles:          { label: 'Azure Files 存储', group: '云存储' },
  'google cloud storage': { label: 'Google Cloud Storage（GCS）', group: '云存储' },
  b2:                  { label: 'Backblaze B2', group: '云存储' },
  swift:               { label: 'OpenStack Swift', group: '云存储' },
  dropbox:             { label: 'Dropbox', group: '网盘' },
  onedrive:            { label: 'Microsoft OneDrive', group: '网盘' },
  box:                 { label: 'Box', group: '网盘' },
  pcloud:              { label: 'pCloud', group: '网盘' },
  mega:                { label: 'MEGA', group: '网盘' },
  'google photos':     { label: 'Google Photos', group: '网盘' },
  yandex:              { label: 'Yandex Disk', group: '网盘' },
  pikpak:              { label: 'PikPak', group: '网盘' },
  iclouddrive:         { label: 'iCloud Drive', group: '网盘' },
  jottacloud:          { label: 'Jottacloud', group: '网盘' },
  hidrive:             { label: 'HiDrive', group: '网盘' },
  protondrive:         { label: 'Proton Drive', group: '网盘' },
  mailru:              { label: 'Mail.ru Cloud', group: '网盘' },
  sugarsync:           { label: 'SugarSync', group: '网盘' },
  putio:               { label: 'Put.io', group: '网盘' },
  zoho:                { label: 'Zoho WorkDrive', group: '网盘' },
  internxt:            { label: 'Internxt Drive', group: '网盘' },
  seafile:             { label: 'Seafile', group: '自建存储' },
  storj:               { label: 'Storj 去中心化存储', group: '云存储' },
  hdfs:                { label: 'Hadoop HDFS', group: '企业存储' },
  oracleobjectstorage: { label: 'Oracle 对象存储', group: '云存储' },
  qingstor:            { label: '青云 QingStor', group: '云存储' },
  sharefile:           { label: 'Citrix ShareFile', group: '企业存储' },
  filefabric:          { label: 'Enterprise File Fabric', group: '企业存储' },
  netstorage:          { label: 'Akamai NetStorage', group: '企业存储' },
  sia:                 { label: 'Sia 去中心化存储', group: '云存储' },
  koofr:               { label: 'Koofr / Digi Storage', group: '网盘' },
  opendrive:           { label: 'OpenDrive', group: '网盘' },
}

/** 构建完整类型选项列表（内置 + rclone，去重+分类） */
export function buildAllTypeOptions(rcloneBackends: { name: string; description: string }[]): TypeOption[] {
  const result = [...BUILTIN_OPTIONS]
  const existingValues = new Set(BUILTIN_OPTIONS.map((o) => o.value))

  for (const backend of rcloneBackends) {
    if (EXCLUDED_BACKENDS.has(backend.name) || existingValues.has(backend.name)) continue
    // 也排除和内置类型实际是同一后端的（如 rclone 的 s3, ftp, webdav 已被内置覆盖）
    existingValues.add(backend.name)

    const meta = RCLONE_LABELS[backend.name]
    result.push({
      label: meta?.label ?? `${backend.name} — ${backend.description}`,
      value: backend.name,
      group: meta?.group ?? '其他',
    })
  }

  return result
}

// ---------------------------------------------------------------------------
// 类型标签
// ---------------------------------------------------------------------------

const TYPE_LABELS: Record<string, string> = {
  local_disk: '本地磁盘', google_drive: 'Google Drive', s3: 'S3 Compatible',
  webdav: 'WebDAV', aliyun_oss: '阿里云 OSS', tencent_cos: '腾讯云 COS',
  qiniu_kodo: '七牛云 Kodo', ftp: 'FTP',
  sftp: 'SFTP', smb: 'SMB', azureblob: 'Azure Blob', dropbox: 'Dropbox',
  onedrive: 'OneDrive', b2: 'Backblaze B2', mega: 'MEGA', pcloud: 'pCloud',
  box: 'Box', swift: 'Swift', pikpak: 'PikPak',
}

export function getStorageTargetTypeLabel(type: StorageTargetType): string {
  return TYPE_LABELS[type] || type.toUpperCase()
}
