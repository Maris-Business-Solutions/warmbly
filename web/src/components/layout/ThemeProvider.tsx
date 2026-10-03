import { useEffect } from 'react'
import { useAppStore } from '@/stores'
import { onSystemThemeChange, readStoredTheme, resolveTheme, THEME_STORAGE_KEY } from '@/lib/theme'

// Keeps the painted theme in step with the preference: the OS when it is
// "system", and the choice made in another tab of the dashboard.
export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const theme = useAppStore((state) => state.theme)
  const setResolvedTheme = useAppStore((state) => state.setResolvedTheme)

  useEffect(() => {
    setResolvedTheme(resolveTheme(theme))
    if (theme !== 'system') return
    return onSystemThemeChange((t) => setResolvedTheme(t))
  }, [theme, setResolvedTheme])

  useEffect(() => {
    // Read the key now (a queued event's newValue can be stale) and never write it back.
    const onStorage = (e: StorageEvent) => {
      if (e.key !== THEME_STORAGE_KEY && e.key !== null) return
      useAppStore.setState({ theme: readStoredTheme() })
    }
    window.addEventListener('storage', onStorage)
    return () => window.removeEventListener('storage', onStorage)
  }, [])

  return <>{children}</>
}
