import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { StorageTargetName } from './StorageTargetName'

describe('StorageTargetName', () => {
  it('uses an SVG icon for starred targets without character symbols', () => {
    const { container } = render(<StorageTargetName name="Central storage" starred />)

    expect(screen.getByText('Central storage')).toBeInTheDocument()
    expect(screen.getByLabelText('Central storage，已收藏')).toBeInTheDocument()
    expect(container.querySelector('svg')).not.toBeNull()
    expect(container.textContent).not.toContain(String.fromCodePoint(0x2605))
  })
})
