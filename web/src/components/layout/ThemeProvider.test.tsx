import { act, render } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useAppStore } from '@/stores'
import { THEME_STORAGE_KEY } from '@/lib/theme'
import { ThemeProvider } from './ThemeProvider'

// setupTests stubs localStorage with mocks, so the stored value is driven through them.
const stored = (value: string | null) => vi.mocked(localStorage.getItem).mockImplementation((k) => (k === THEME_STORAGE_KEY ? value : null))

describe('ThemeProvider', () => {
  afterEach(() => {
    vi.mocked(localStorage.getItem).mockReset()
    vi.mocked(localStorage.setItem).mockClear()
    useAppStore.setState({ theme: 'light', resolvedTheme: 'light' })
    document.documentElement.classList.remove('dark')
  })

  it("applies another tab's choice from storage without writing it back", () => {
    render(<ThemeProvider><div /></ThemeProvider>)
    stored('dark')
    vi.mocked(localStorage.setItem).mockClear()
    // A queued event can carry a value that is no longer current.
    act(() => {
      window.dispatchEvent(new StorageEvent('storage', { key: THEME_STORAGE_KEY, newValue: 'light' }))
    })
    expect(useAppStore.getState().theme).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(localStorage.setItem).not.toHaveBeenCalledWith(THEME_STORAGE_KEY, expect.anything())
  })

  it('falls back to light when nothing is stored', () => {
    useAppStore.setState({ theme: 'dark', resolvedTheme: 'dark' })
    render(<ThemeProvider><div /></ThemeProvider>)
    stored(null)
    act(() => {
      window.dispatchEvent(new StorageEvent('storage', { key: THEME_STORAGE_KEY, newValue: null }))
    })
    expect(useAppStore.getState().theme).toBe('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })
})
