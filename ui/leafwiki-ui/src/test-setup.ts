import '@testing-library/jest-dom'

if (typeof window !== 'undefined' && !window.localStorage) {
  const storage = new Map<string, string>()

  Object.defineProperty(window, 'localStorage', {
    value: {
      clear: () => storage.clear(),
      getItem: (key: string) => storage.get(key) ?? null,
      removeItem: (key: string) => storage.delete(key),
      setItem: (key: string, value: string) => {
        storage.set(key, value)
      },
    },
    configurable: true,
  })
}
