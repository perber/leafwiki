import { useDesignModeStore } from '@/features/designtoggle/designmode'
import { useLayoutEffect, useSyncExternalStore } from 'react'

function applyDesignMode(mode: 'light' | 'dark' | 'system') {
  const root = document.documentElement

  let appliedMode: 'light' | 'dark'
  if (mode === 'system') {
    const prefersLight = window.matchMedia(
      '(prefers-color-scheme: light)',
    ).matches
    appliedMode = prefersLight ? 'light' : 'dark'
  } else {
    appliedMode = mode
  }

  if (appliedMode === 'dark') {
    root.classList.add('dark')
  } else {
    root.classList.remove('dark')
  }
}

export default function useApplyDesignMode() {
  const mode = useDesignModeStore((s) => s.mode)
  const prefersLight = useSyncExternalStore(
    (onStoreChange) => {
      if (typeof window === 'undefined' || mode !== 'system') {
        return () => {}
      }

      const mediaQuery = window.matchMedia('(prefers-color-scheme: light)')
      mediaQuery.addEventListener('change', onStoreChange)
      return () => {
        mediaQuery.removeEventListener('change', onStoreChange)
      }
    },
    () => {
      if (typeof window === 'undefined') return true
      return window.matchMedia('(prefers-color-scheme: light)').matches
    },
    () => true,
  )

  useLayoutEffect(() => {
    applyDesignMode(mode)
  }, [mode, prefersLight])
}
