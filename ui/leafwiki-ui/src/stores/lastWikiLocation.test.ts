import { beforeEach, describe, expect, it } from 'vitest'
import { useLastWikiLocationStore } from './lastWikiLocation'

beforeEach(() => {
  useLastWikiLocationStore.setState({ location: null })
})

describe('useLastWikiLocationStore', () => {
  it('starts with no remembered location', () => {
    expect(useLastWikiLocationStore.getState().location).toBeNull()
  })

  it('stores and returns a location including its state', () => {
    const loc = {
      pathname: '/docs/getting-started',
      search: '?x=1',
      state: { leafwikiVisitId: 'abc-123' },
    }

    useLastWikiLocationStore.getState().setLocation(loc)

    expect(useLastWikiLocationStore.getState().location).toEqual(loc)
  })

  it('can be reset back to null', () => {
    useLastWikiLocationStore.getState().setLocation({
      pathname: '/a',
      search: '',
      state: null,
    })

    useLastWikiLocationStore.getState().setLocation(null)

    expect(useLastWikiLocationStore.getState().location).toBeNull()
  })
})
