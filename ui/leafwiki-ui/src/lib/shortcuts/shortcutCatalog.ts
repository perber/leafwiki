import i18next from '@/lib/i18n'
import { AppMode } from '@/lib/useAppMode'
import { HotKeyDefinition } from '@/stores/hotkeys'

export type ShortcutId =
  | 'dialog.close'
  | 'dialog.confirm'
  | 'page.quickSwitcher.open'
  | 'sidebar.explorer.open'
  | 'sidebar.search.open'
  | 'viewer.page.edit'
  | 'viewer.page.permalink'
  | 'viewer.page.print'
  | 'viewer.page.copy'
  | 'viewer.page.delete'
  | 'viewer.page.history'
  | 'editor.page.close'
  | 'editor.page.save'
  | 'editor.format.bold'
  | 'editor.format.italic'
  | 'editor.heading.one'
  | 'editor.heading.two'
  | 'editor.heading.three'
  | 'editor.format.inlineCode'
  | 'editor.link.insert'
  | 'history.page.close'
  | 'asset.rename.confirm'
  | 'asset.rename.cancel'

export type ShortcutDefinition = {
  id: ShortcutId
  labelKey: string
  categoryKey: string
  keyCombo: string
  defaultDisplayLabel: string
  macDisplayLabel?: string
  modes: AppMode[]
  customizable: boolean
}

export const shortcutDefinitions: ShortcutDefinition[] = [
  {
    id: 'dialog.close',
    labelKey: 'shortcutsHelp.items.closeDialog.action',
    categoryKey: 'shortcutsHelp.categories.dialogs',
    keyCombo: 'Escape',
    defaultDisplayLabel: 'Esc',
    modes: ['dialog'],
    customizable: false,
  },
  {
    id: 'dialog.confirm',
    labelKey: 'shortcutsHelp.items.confirmDialog.action',
    categoryKey: 'shortcutsHelp.categories.dialogs',
    keyCombo: 'Enter',
    defaultDisplayLabel: 'Enter',
    modes: ['dialog'],
    customizable: false,
  },
  {
    id: 'page.quickSwitcher.open',
    labelKey: 'shortcutsHelp.items.goToPage.action',
    categoryKey: 'shortcutsHelp.categories.navigation',
    keyCombo: 'Mod+Alt+KeyP',
    defaultDisplayLabel: 'Ctrl+Alt+P',
    macDisplayLabel: 'Cmd+Option+P',
    modes: ['view'],
    customizable: true,
  },
  {
    id: 'sidebar.explorer.open',
    labelKey: 'shortcutsHelp.items.openExplorer.action',
    categoryKey: 'shortcutsHelp.categories.navigation',
    keyCombo: 'Mod+Shift+KeyE',
    defaultDisplayLabel: 'Ctrl+Shift+E',
    macDisplayLabel: 'Cmd+Shift+E',
    modes: ['view', 'edit', 'history', 'settings', 'user-management'],
    customizable: true,
  },
  {
    id: 'sidebar.search.open',
    labelKey: 'shortcutsHelp.items.openSearch.action',
    categoryKey: 'shortcutsHelp.categories.navigation',
    keyCombo: 'Mod+Shift+KeyF',
    defaultDisplayLabel: 'Ctrl+Shift+F',
    macDisplayLabel: 'Cmd+Shift+F',
    modes: ['view', 'edit', 'history', 'settings', 'user-management'],
    customizable: true,
  },
  {
    id: 'viewer.page.edit',
    labelKey: 'shortcutsHelp.items.editPage.action',
    categoryKey: 'shortcutsHelp.categories.viewing',
    keyCombo: 'Mod+KeyE',
    defaultDisplayLabel: 'Ctrl+E',
    macDisplayLabel: 'Cmd+E',
    modes: ['view'],
    customizable: true,
  },
  {
    id: 'viewer.page.permalink',
    labelKey: 'shortcutsHelp.items.sharePage.action',
    categoryKey: 'shortcutsHelp.categories.viewing',
    keyCombo: 'Mod+Shift+KeyL',
    defaultDisplayLabel: 'Ctrl+Shift+L',
    macDisplayLabel: 'Cmd+Shift+L',
    modes: ['view'],
    customizable: true,
  },
  {
    id: 'viewer.page.print',
    labelKey: 'shortcutsHelp.items.printPage.action',
    categoryKey: 'shortcutsHelp.categories.viewing',
    keyCombo: 'Mod+KeyP',
    defaultDisplayLabel: 'Ctrl+P',
    macDisplayLabel: 'Cmd+P',
    modes: ['view'],
    customizable: true,
  },
  {
    id: 'viewer.page.copy',
    labelKey: 'shortcutsHelp.items.copyPage.action',
    categoryKey: 'shortcutsHelp.categories.viewing',
    keyCombo: 'Mod+Shift+KeyS',
    defaultDisplayLabel: 'Ctrl+Shift+S',
    macDisplayLabel: 'Cmd+Shift+S',
    modes: ['view'],
    customizable: true,
  },
  {
    id: 'viewer.page.delete',
    labelKey: 'shortcutsHelp.items.deletePage.action',
    categoryKey: 'shortcutsHelp.categories.viewing',
    keyCombo: 'Mod+Delete',
    defaultDisplayLabel: 'Ctrl+Delete',
    macDisplayLabel: 'Cmd+Delete',
    modes: ['view'],
    customizable: true,
  },
  {
    id: 'viewer.page.history',
    labelKey: 'shortcutsHelp.items.pageHistory.action',
    categoryKey: 'shortcutsHelp.categories.viewing',
    keyCombo: 'Mod+KeyH',
    defaultDisplayLabel: 'Ctrl+H',
    macDisplayLabel: 'Cmd+H',
    modes: ['view'],
    customizable: true,
  },
  {
    id: 'editor.page.close',
    labelKey: 'shortcutsHelp.items.closeEditor.action',
    categoryKey: 'shortcutsHelp.categories.editing',
    keyCombo: 'Escape',
    defaultDisplayLabel: 'Esc',
    modes: ['edit'],
    customizable: false,
  },
  {
    id: 'editor.page.save',
    labelKey: 'shortcutsHelp.items.savePage.action',
    categoryKey: 'shortcutsHelp.categories.editing',
    keyCombo: 'Mod+KeyS',
    defaultDisplayLabel: 'Ctrl+S',
    macDisplayLabel: 'Cmd+S',
    modes: ['edit'],
    customizable: true,
  },
  {
    id: 'editor.format.bold',
    labelKey: 'shortcutsHelp.items.formatBold.action',
    categoryKey: 'shortcutsHelp.categories.editing',
    keyCombo: 'Mod+KeyB',
    defaultDisplayLabel: 'Ctrl+B',
    macDisplayLabel: 'Cmd+B',
    modes: ['edit'],
    customizable: true,
  },
  {
    id: 'editor.format.italic',
    labelKey: 'shortcutsHelp.items.formatItalic.action',
    categoryKey: 'shortcutsHelp.categories.editing',
    keyCombo: 'Mod+KeyI',
    defaultDisplayLabel: 'Ctrl+I',
    macDisplayLabel: 'Cmd+I',
    modes: ['edit'],
    customizable: true,
  },
  {
    id: 'editor.heading.one',
    labelKey: 'shortcutsHelp.items.insertHeadingOne.action',
    categoryKey: 'shortcutsHelp.categories.editing',
    keyCombo: 'Mod+Alt+Digit1',
    defaultDisplayLabel: 'Ctrl+Alt+1',
    macDisplayLabel: 'Cmd+Option+1',
    modes: ['edit'],
    customizable: true,
  },
  {
    id: 'editor.heading.two',
    labelKey: 'shortcutsHelp.items.insertHeadingTwo.action',
    categoryKey: 'shortcutsHelp.categories.editing',
    keyCombo: 'Mod+Alt+Digit2',
    defaultDisplayLabel: 'Ctrl+Alt+2',
    macDisplayLabel: 'Cmd+Option+2',
    modes: ['edit'],
    customizable: true,
  },
  {
    id: 'editor.heading.three',
    labelKey: 'shortcutsHelp.items.insertHeadingThree.action',
    categoryKey: 'shortcutsHelp.categories.editing',
    keyCombo: 'Mod+Alt+Digit3',
    defaultDisplayLabel: 'Ctrl+Alt+3',
    macDisplayLabel: 'Cmd+Option+3',
    modes: ['edit'],
    customizable: true,
  },
  {
    id: 'editor.format.inlineCode',
    labelKey: 'shortcutsHelp.items.formatInlineCode.action',
    categoryKey: 'shortcutsHelp.categories.editing',
    keyCombo: 'Mod+Backquote',
    defaultDisplayLabel: 'Ctrl+`',
    macDisplayLabel: 'Cmd+`',
    modes: ['edit'],
    customizable: true,
  },
  {
    id: 'editor.link.insert',
    labelKey: 'shortcutsHelp.items.insertLink.action',
    categoryKey: 'shortcutsHelp.categories.editing',
    keyCombo: 'Mod+KeyK',
    defaultDisplayLabel: 'Ctrl+K',
    macDisplayLabel: 'Cmd+K',
    modes: ['edit'],
    customizable: true,
  },
  {
    id: 'history.page.close',
    labelKey: 'shortcutsHelp.items.backToPage.action',
    categoryKey: 'shortcutsHelp.categories.navigation',
    keyCombo: 'Escape',
    defaultDisplayLabel: 'Esc',
    modes: ['history'],
    customizable: false,
  },
  {
    id: 'asset.rename.confirm',
    labelKey: 'shortcutsHelp.items.confirmRename.action',
    categoryKey: 'shortcutsHelp.categories.dialogs',
    keyCombo: 'Enter',
    defaultDisplayLabel: 'Enter',
    modes: ['dialog'],
    customizable: false,
  },
  {
    id: 'asset.rename.cancel',
    labelKey: 'shortcutsHelp.items.cancelRename.action',
    categoryKey: 'shortcutsHelp.categories.dialogs',
    keyCombo: 'Escape',
    defaultDisplayLabel: 'Esc',
    modes: ['dialog'],
    customizable: false,
  },
]

const shortcutDefinitionMap = new Map(
  shortcutDefinitions.map((definition) => [definition.id, definition]),
)

export function getShortcutDefinition(id: ShortcutId): ShortcutDefinition {
  const definition = shortcutDefinitionMap.get(id)

  if (!definition) {
    throw new Error(`Shortcut definition for "${id}" not found.`)
  }

  return definition
}

export function getVisibleShortcutsForMode(mode: AppMode): ShortcutDefinition[] {
  return shortcutDefinitions
    .filter((definition) => definition.modes.includes(mode))
    .sort((left, right) => {
      if (left.modes.length !== right.modes.length) {
        return left.modes.length - right.modes.length
      }

      return left.labelKey.localeCompare(right.labelKey)
    })
}

export function getShortcutDisplayLabel(id: ShortcutId, isMacOS: boolean) {
  const definition = getShortcutDefinition(id)
  return isMacOS && definition.macDisplayLabel
    ? definition.macDisplayLabel
    : definition.defaultDisplayLabel
}

export function createHotkeyDefinition(
  id: ShortcutId,
  action: () => void,
  options?: Partial<Pick<HotKeyDefinition, 'enabled' | 'shouldHandle'>>,
): HotKeyDefinition {
  const definition = getShortcutDefinition(id)

  return {
    keyCombo: definition.keyCombo,
    enabled: options?.enabled ?? true,
    mode: definition.modes,
    shouldHandle: options?.shouldHandle,
    action,
  }
}

export function getShortcutLabel(id: ShortcutId) {
  return i18next.t(getShortcutDefinition(id).labelKey, { ns: 'viewer' })
}
