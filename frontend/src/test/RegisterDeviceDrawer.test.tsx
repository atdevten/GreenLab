import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { RegisterDeviceDrawer } from '../components/ui/RegisterDeviceDrawer'

const noop = () => {}

describe('RegisterDeviceDrawer', () => {
  it('renders location fields in step 1', () => {
    render(
      <RegisterDeviceDrawer
        open
        onClose={noop}
        onRegister={vi.fn().mockResolvedValue({ channelId: 'c1', apiKey: 'k1' })}
        workspaces={[{ id: 'ws1', name: 'Default', slug: 'default', device_count: 0, channel_count: 0, member_count: 0, created_at: '' }]}
      />
    )
    expect(screen.getByPlaceholderText(/10\.7769/)).toBeInTheDocument()
    expect(screen.getByPlaceholderText(/106\.7009/)).toBeInTheDocument()
    expect(screen.getByPlaceholderText(/Greenhouse A/)).toBeInTheDocument()
  })

  it('location inputs start empty', () => {
    render(
      <RegisterDeviceDrawer
        open
        onClose={noop}
        onRegister={vi.fn().mockResolvedValue({ channelId: 'c1', apiKey: 'k1' })}
        workspaces={[{ id: 'ws1', name: 'Default', slug: 'default', device_count: 0, channel_count: 0, member_count: 0, created_at: '' }]}
      />
    )
    const latInput = screen.getByPlaceholderText(/10\.7769/) as HTMLInputElement
    const lngInput = screen.getByPlaceholderText(/106\.7009/) as HTMLInputElement
    expect(latInput.value).toBe('')
    expect(lngInput.value).toBe('')
  })
})
