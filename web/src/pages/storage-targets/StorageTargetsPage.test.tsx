import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { StorageTargetsPage } from './StorageTargetsPage'
import { getStorageTargetUsage, listStorageTargets } from '../../services/storage-targets'

vi.mock('../../services/storage-targets', () => ({
  listStorageTargets: vi.fn(),
  getStorageTargetUsage: vi.fn(),
  createStorageTarget: vi.fn(),
  deleteStorageTarget: vi.fn(),
  getStorageTarget: vi.fn(),
  startGoogleDriveAuth: vi.fn(),
  testSavedStorageTarget: vi.fn(),
  testStorageTarget: vi.fn(),
  toggleStorageTargetStar: vi.fn(),
  updateStorageTarget: vi.fn(),
}))
vi.mock('../../components/storage-targets/StorageTargetFormDrawer', () => ({
  StorageTargetFormDrawer: () => null,
}))

afterEach(cleanup)

function mockTarget(quotaBytes = 0) {
  vi.mocked(listStorageTargets).mockResolvedValue([
    {
      id: 1,
      name: 'MinIO',
      type: 's3',
      description: '',
      enabled: true,
      starred: false,
      updatedAt: '2026-10-06',
      lastTestStatus: 'success',
      quotaBytes,
    },
  ])
}

describe('storage usage', () => {
  it('shows zero tracked bytes and explains missing physical capacity', async () => {
    mockTarget()
    vi.mocked(getStorageTargetUsage).mockResolvedValue({
      targetId: 1,
      targetName: 'MinIO',
      recordCount: 0,
      totalSize: 0,
    })
    render(<StorageTargetsPage />)
    expect(await screen.findByText('备份记录大小：0 B')).toBeInTheDocument()
    expect(screen.getByText(/未获取到存储总容量/)).toBeInTheDocument()
    expect(screen.queryByText(/使用率.*%/)).not.toBeInTheDocument()
  })

  it('labels the configured backup quota without pretending it is disk capacity', async () => {
    mockTarget(1024)
    vi.mocked(getStorageTargetUsage).mockResolvedValue({
      targetId: 1,
      targetName: 'MinIO',
      recordCount: 0,
      totalSize: 512,
    })
    render(<StorageTargetsPage />)
    expect(await screen.findByText('备份配额使用率 50%')).toBeInTheDocument()
    expect(screen.getByText(/不代表存储物理容量/)).toBeInTheDocument()
    expect(screen.queryByText(/0 个记录/)).not.toBeInTheDocument()
  })

  it('uses backend capacity in preference to the backup quota', async () => {
    mockTarget(1024)
    vi.mocked(getStorageTargetUsage).mockResolvedValue({
      targetId: 1,
      targetName: 'MinIO',
      recordCount: 0,
      totalSize: 512,
      diskUsage: { total: 4096, used: 1024 },
    })
    render(<StorageTargetsPage />)
    expect(await screen.findByText('使用率 25%')).toBeInTheDocument()
    expect(screen.queryByText(/备份配额使用率/)).not.toBeInTheDocument()
  })
})
