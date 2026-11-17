// The ?ssr parameter makes the server put the page it renders for search
// engines into #root (internal/wiki/seo, ssrPreviewParam). The SPA's
// createRoot would replace it at once, so with the parameter the SPA stays
// off and the server's markup stays on screen for checking.
//
// Only when the server sent a body: in a private wiki, on an unknown page or
// an SPA route #root holds no element (the Vite dev server leaves the bare
// placeholder text) and the SPA mounts as usual.

export const SSR_PREVIEW_PARAM = 'ssr'

export function isSSRPreview(search: string, root: HTMLElement): boolean {
  return (
    new URLSearchParams(search).has(SSR_PREVIEW_PARAM) &&
    root.firstElementChild !== null
  )
}
