import type { NotificationFieldConfig, NotificationType } from '../../types/notifications'

const FIELD_CONFIG_MAP: Record<NotificationType, NotificationFieldConfig[]> = {
  email: [
    { key: 'host', label: 'SMTP Host', type: 'input', required: true, placeholder: 'smtp.example.com' },
    { key: 'port', label: 'SMTP Port', type: 'number', required: true, placeholder: '587' },
    { key: 'username', label: '用户名', type: 'input', placeholder: '可选' },
    { key: 'password', label: '密码', type: 'password', placeholder: '留空表示保持原密码', sensitive: true },
    { key: 'from', label: '发件人', type: 'input', required: true, placeholder: 'backupx@example.com' },
    { key: 'to', label: '收件人', type: 'input', required: true, placeholder: 'ops@example.com,dev@example.com' },
  ],
  webhook: [
    { key: 'url', label: 'Webhook URL', type: 'input', required: true, placeholder: 'https://hooks.example.com/backupx' },
    { key: 'secret', label: '共享密钥', type: 'password', placeholder: '可选', sensitive: true },
  ],
  sms: [
    { key: 'url', label: 'SMS Webhook URL', type: 'input', required: true, placeholder: 'https://sms-gateway.example.com/send' },
    { key: 'secret', label: '共享密钥', type: 'password', placeholder: '可选', sensitive: true },
  ],
  telegram: [
    { key: 'botToken', label: 'Bot Token', type: 'password', required: true, placeholder: '123456:ABC', sensitive: true },
    { key: 'chatId', label: 'Chat ID', type: 'input', required: true, placeholder: '-100xxxxxxxxxx' },
  ],
}

export const notificationTypeOptions = [
  { label: 'Email', value: 'email' },
  { label: 'Webhook', value: 'webhook' },
  { label: 'Telegram', value: 'telegram' },
  { label: 'SMS Webhook', value: 'sms' },
] as const

export function getNotificationTypeLabel(type: NotificationType) {
  switch (type) {
    case 'email':
      return 'Email'
    case 'webhook':
      return 'Webhook'
    case 'telegram':
      return 'Telegram'
    case 'sms':
      return 'SMS Webhook'
    default:
      return type
  }
}

export function getNotificationFieldConfigs(type: NotificationType) {
  return FIELD_CONFIG_MAP[type]
}
