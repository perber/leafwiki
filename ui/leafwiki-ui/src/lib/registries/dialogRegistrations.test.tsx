import { isValidElement } from 'react'
import { describe, expect, it } from 'vitest'
import {
  DIALOG_ADD_PAGE,
  DIALOG_API_KEY_FORM,
  DIALOG_PAGE_REFACTOR_CONFIRMATION,
  DIALOG_RESTORE_REVISION_CONFIRMATION,
  dialogRegistry,
} from './index'

function renderDialog(type: string, props: unknown) {
  const dialog = dialogRegistry.getDialog(type)
  if (!dialog) throw new Error(`dialog ${type} not registered`)
  const element = dialog.render(props)
  if (!isValidElement(element)) throw new Error('expected a React element')
  return element as React.ReactElement<Record<string, unknown>>
}

describe('application dialog registrations', () => {
  it('registers every application dialog type in a stable order', () => {
    expect(dialogRegistry.getAllDialogs().map((d) => d.type)).toEqual([
      'add-page',
      'sort-pages',
      'move-page',
      'create-page-by-path',
      'copy-page',
      'edit-page-metadata',
      'asset-manager',
      'delete-page-confirmation',
      'user-form',
      'change-user-password',
      'delete-user-confirmation',
      'api-key-form',
      'delete-api-key-confirmation',
      'unsaved-changes',
      'image-preview',
      'page-quick-switcher',
      'shortcuts-help',
      'page-refactor-confirmation',
      'page-permalink',
      'link-insert',
      'restore-revision-confirmation',
      'wikilink-disambiguation',
    ])
  })

  it('keys a dialog by its type and forwards the opener props', () => {
    const element = renderDialog(DIALOG_ADD_PAGE, { parentId: 'p1' })
    expect(element.key).toBe(DIALOG_ADD_PAGE)
    expect(element.props).toEqual({ parentId: 'p1' })
  })

  it('does not forward props to dialogs that take none', () => {
    const element = renderDialog(DIALOG_API_KEY_FORM, { stray: true })
    expect(element.key).toBe(DIALOG_API_KEY_FORM)
    expect(element.props).toEqual({})
  })

  it('keys the refactor and restore dialogs by their subject', () => {
    const refactor = renderDialog(DIALOG_PAGE_REFACTOR_CONFIRMATION, {
      preview: { pageId: 'abc', newPath: '/new' },
    })
    expect(refactor.key).toBe(`${DIALOG_PAGE_REFACTOR_CONFIRMATION}-abc-/new`)

    const restore = renderDialog(DIALOG_RESTORE_REVISION_CONFIRMATION, {
      revision: { id: 'rev-1' },
    })
    expect(restore.key).toBe(`${DIALOG_RESTORE_REVISION_CONFIRMATION}-rev-1`)
    expect(restore.props).toEqual({ revision: { id: 'rev-1' } })
  })
})
