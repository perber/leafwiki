import { describe, expect, it } from 'vitest'
import {
  createHotkeyDefinition,
  getShortcutDefinition,
  getShortcutDisplayLabel,
  getVisibleShortcutsForMode,
  shortcutDefinitions,
} from './shortcutCatalog'

describe('shortcutCatalog', () => {
  it('defines unique shortcut ids', () => {
    const ids = shortcutDefinitions.map((definition) => definition.id)
    expect(new Set(ids).size).toBe(ids.length)
  })

  it('returns the registered definition for a known shortcut', () => {
    expect(getShortcutDefinition('page.quickSwitcher.open')).toMatchObject({
      id: 'page.quickSwitcher.open',
      keyCombo: 'Mod+Alt+KeyP',
      modes: ['view'],
      labelKey: 'shortcutsHelp.items.goToPage.action',
    })
  })

  it('filters visible shortcuts by the current mode', () => {
    expect(getVisibleShortcutsForMode('view').map((item) => item.id)).toEqual([
      'viewer.page.copy',
      'viewer.page.delete',
      'viewer.page.edit',
      'page.quickSwitcher.open',
      'viewer.page.history',
      'viewer.page.print',
      'viewer.page.permalink',
      'sidebar.explorer.open',
      'sidebar.search.open',
    ])

    expect(getVisibleShortcutsForMode('edit').map((item) => item.id)).toEqual([
      'editor.page.close',
      'editor.format.bold',
      'editor.format.inlineCode',
      'editor.format.italic',
      'editor.heading.one',
      'editor.heading.three',
      'editor.heading.two',
      'editor.link.insert',
      'editor.page.save',
      'sidebar.explorer.open',
      'sidebar.search.open',
    ])

    expect(getVisibleShortcutsForMode('history').map((item) => item.id)).toEqual(
      [
        'history.page.close',
        'sidebar.explorer.open',
        'sidebar.search.open',
      ],
    )
  })

  it('includes the remaining viewer and editor shortcuts in the catalog', () => {
    expect(getShortcutDefinition('viewer.page.edit').keyCombo).toBe('Mod+KeyE')
    expect(getShortcutDefinition('viewer.page.history').keyCombo).toBe(
      'Mod+KeyH',
    )
    expect(getShortcutDefinition('editor.format.bold').keyCombo).toBe(
      'Mod+KeyB',
    )
    expect(getShortcutDefinition('editor.link.insert').keyCombo).toBe(
      'Mod+KeyK',
    )
  })

  it('returns platform-specific display labels', () => {
    expect(getShortcutDisplayLabel('page.quickSwitcher.open', false)).toBe(
      'Ctrl+Alt+P',
    )
    expect(getShortcutDisplayLabel('page.quickSwitcher.open', true)).toBe(
      'Cmd+Option+P',
    )
    expect(getShortcutDisplayLabel('sidebar.explorer.open', false)).toBe(
      'Ctrl+Shift+E',
    )
    expect(getShortcutDisplayLabel('viewer.page.delete', false)).toBe(
      'Ctrl+Delete',
    )
    expect(getShortcutDisplayLabel('editor.heading.one', true)).toBe(
      'Cmd+Option+1',
    )
  })

  it('creates a hotkey definition with the catalog combo and modes', () => {
    const hotkey = createHotkeyDefinition('history.page.close', () => undefined)

    expect(hotkey).toMatchObject({
      keyCombo: 'Escape',
      enabled: true,
      mode: ['history'],
    })
  })

  it('supports generic dialog and asset rename shortcuts', () => {
    expect(getShortcutDefinition('dialog.close').keyCombo).toBe('Escape')
    expect(getShortcutDefinition('dialog.confirm').keyCombo).toBe('Enter')
    expect(getShortcutDefinition('asset.rename.confirm').keyCombo).toBe('Enter')
    expect(getShortcutDefinition('asset.rename.cancel').keyCombo).toBe('Escape')
  })
})
