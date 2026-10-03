import { useEffect } from 'react'
import { useAppStore } from '@/stores'
import { DEFAULT_THEME, isTheme, onSystemThemeChange, resolveTheme, THEME_STORAGE_KEY } from '@/lib/theme'

// Keeps the painted theme in step with the preference: the OS when it is
// "system", and the choice made in another tab of the dashboard.
export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const theme = useAppStore((state) => state.theme)
  const setTheme = useAppStore((state) => state.setTheme)
  const setResolvedTheme = useAppStore((state) => state.setResolvedTheme)

  useEffect(() => {
    setResolvedTheme(resolveTheme(theme))
    if (theme !== 'system') return
    return onSystemThemeChange((t) => setResolvedTheme(t))
  }, [theme, setResolvedTheme])

  useEffect(() => {
    const onStorage = (e: StorageEvent) => {
      if (e.key !== THEME_STORAGE_KEY) return
      setTheme(isTheme(e.newValue) ? e.newValue : DEFAULT_THEME)
    }
    window.addEventListener('storage', onStorage)
    return () => window.removeEventListener('storage', onStorage)
  }, [setTheme])

  return <>{children}</>
}
