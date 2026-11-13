import { beforeEach, describe, expect, it } from 'vitest'
import { useEditorStore } from './editor'

describe('useEditorStore spell check', () => {
  beforeEach(() => {
    localStorage.clear()
    useEditorStore.setState({ spellcheck: true })
  })

  it('turns the browser spell check on by default', () => {
    expect(useEditorStore.getInitialState().spellcheck).toBe(true)
  })

  it('toggles the spell check on and off', () => {
    useEditorStore.getState().toggleSpellcheck()
    expect(useEditorStore.getState().spellcheck).toBe(false)

    useEditorStore.getState().toggleSpellcheck()
    expect(useEditorStore.getState().spellcheck).toBe(true)
  })

  it('remembers the spell check choice across reloads', () => {
    useEditorStore.getState().toggleSpellcheck()

    const persisted = JSON.parse(
      localStorage.getItem('leafwiki-editor-settings') ?? '{}',
    )
    expect(persisted.state.spellcheck).toBe(false)
  })
})
