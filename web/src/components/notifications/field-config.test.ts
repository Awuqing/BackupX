import { describe, expect, it } from 'vitest'
import { getNotificationFieldConfigs, getNotificationTypeLabel } from './field-config'

describe('notification field config', () => {
  it('returns readable type labels', () => {
    expect(getNotificationTypeLabel('email')).toBe('Email')
    expect(getNotificationTypeLabel('telegram')).toBe('Telegram')
  })

  it('returns required fields for each notification type', () => {
    const emailFields = getNotificationFieldConfigs('email')
    const webhookFields = getNotificationFieldConfigs('webhook')
    const telegramFields = getNotificationFieldConfigs('telegram')

    expect(emailFields.some((field) => field.key === 'host' && field.required)).toBe(true)
    expect(webhookFields.some((field) => field.key === 'url' && field.required)).toBe(true)
    expect(telegramFields.some((field) => field.key === 'botToken' && field.required)).toBe(true)
  })
})
