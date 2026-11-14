import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { RestoreConfirmModal } from './RestoreConfirmModal'
import type { BackupRecordDetail } from '../../types/backup-records'
import type { BackupTaskDetail } from '../../types/backup-tasks'

afterEach(cleanup)

const record = {
  id: 2,
  fileName: 'logs.pitr.tar',
  backupKind: 'binlog',
  mysqlCheckpoint: {
    baselineAt: '2026-11-14T01:00:00Z',
    capturedAt: '2026-11-14T02:00:00Z',
    full: false,
  },
} as BackupRecordDetail
const task = {
  id: 1,
  name: 'MySQL',
  type: 'mysql',
  dbName: '*',
  dbHost: 'db',
  dbPort: 3306,
  dbUser: 'backup',
} as BackupTaskDetail

describe('MySQL point-in-time restore', () => {
  it('converts an explicit timezone to UTC before starting restore', () => {
    const onConfirm = vi.fn()
    render(
      <RestoreConfirmModal
        visible
        loading={false}
        backupRecord={record}
        task={task}
        onCancel={vi.fn()}
        onConfirm={onConfirm}
      />,
    )
    fireEvent.change(screen.getByLabelText('MySQL 恢复时间'), {
      target: { value: '2026-11-14T09:30:00+08:00' },
    })
    fireEvent.click(screen.getByRole('button', { name: '开始恢复' }))
    expect(onConfirm).toHaveBeenCalledWith(undefined, '2026-11-14T01:30:00.000Z')
  })

  it('rejects missing timezone, out-of-window times and the exclusive endpoint', () => {
    render(
      <RestoreConfirmModal
        visible
        loading={false}
        backupRecord={record}
        task={task}
        onCancel={vi.fn()}
        onConfirm={vi.fn()}
      />,
    )
    for (const value of [
      '2026-11-14T01:30:00',
      '2026-11-14T00:59:59Z',
      '2026-11-14T02:00:00Z',
      'bad',
    ]) {
      fireEvent.change(screen.getByLabelText('MySQL 恢复时间'), { target: { value } })
      expect(screen.getByRole('button', { name: '开始恢复' })).toBeDisabled()
    }
    fireEvent.change(screen.getByLabelText('MySQL 恢复时间'), { target: { value: '' } })
    expect(screen.getByRole('button', { name: '开始恢复' })).toBeEnabled()
  })
})
