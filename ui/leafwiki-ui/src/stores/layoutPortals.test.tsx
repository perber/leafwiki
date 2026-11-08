import { render } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { useLayoutPortalsStore } from './layoutPortals'

// Mirrors how AppLayout hands its portal targets to the store: the setters
// are used directly as callback refs.
function PortalTargets() {
  const setSubheaderRoot = useLayoutPortalsStore((s) => s.setSubheaderRoot)
  const setTocPaneRoot = useLayoutPortalsStore((s) => s.setTocPaneRoot)
  return (
    <>
      <div data-testid="subheader" ref={setSubheaderRoot} />
      <div data-testid="toc" ref={setTocPaneRoot} />
    </>
  )
}

describe('useLayoutPortalsStore', () => {
  beforeEach(() => {
    useLayoutPortalsStore.setState({ subheaderRoot: null, tocPaneRoot: null })
  })

  it('holds the mounted elements and clears them on unmount', () => {
    const { getByTestId, unmount } = render(<PortalTargets />)

    expect(useLayoutPortalsStore.getState().subheaderRoot).toBe(
      getByTestId('subheader'),
    )
    expect(useLayoutPortalsStore.getState().tocPaneRoot).toBe(
      getByTestId('toc'),
    )

    unmount()

    expect(useLayoutPortalsStore.getState().subheaderRoot).toBeNull()
    expect(useLayoutPortalsStore.getState().tocPaneRoot).toBeNull()
  })

  it('points at the new elements after a remount', () => {
    render(<PortalTargets />).unmount()
    const { getByTestId } = render(<PortalTargets />)

    expect(useLayoutPortalsStore.getState().tocPaneRoot).toBe(
      getByTestId('toc'),
    )
    expect(
      (useLayoutPortalsStore.getState().tocPaneRoot as HTMLElement).isConnected,
    ).toBe(true)
  })
})
