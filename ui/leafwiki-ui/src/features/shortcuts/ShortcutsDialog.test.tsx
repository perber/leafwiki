import { DIALOG_SHORTCUTS_HELP } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it } from 'vitest'
import { ShortcutsDialog } from './ShortcutsDialog'

describe('ShortcutsDialog', () => {
  beforeEach(() => {
    useDialogsStore.setState({
      dialogType: DIALOG_SHORTCUTS_HELP,
      dialogProps: null,
    })
  })

  it('shows only view-mode shortcuts on a normal page route', () => {
    render(
      <MemoryRouter initialEntries={['/docs/getting-started']}>
        <ShortcutsDialog />
      </MemoryRouter>,
    )

    expect(screen.getByText('Go to page')).toBeInTheDocument()
    expect(screen.getByText('Open explorer')).toBeInTheDocument()
    expect(screen.getByText('Open search')).toBeInTheDocument()
    expect(screen.getByText('Close dialog')).toBeInTheDocument()
    expect(screen.getByText('Dialog default action')).toBeInTheDocument()
    expect(screen.queryByText('Save page')).not.toBeInTheDocument()
    expect(screen.queryByText('Back to page')).not.toBeInTheDocument()
  })

  it('shows edit-mode shortcuts on an editor route', () => {
    render(
      <MemoryRouter initialEntries={['/e/docs/getting-started']}>
        <ShortcutsDialog />
      </MemoryRouter>,
    )

    expect(screen.getByText('Save page')).toBeInTheDocument()
    expect(screen.getByText('Close editor')).toBeInTheDocument()
    expect(screen.getByText('Open explorer')).toBeInTheDocument()
    expect(screen.getByText('Close dialog')).toBeInTheDocument()
    expect(screen.queryByText('Go to page')).not.toBeInTheDocument()
  })

  it('renders the current mode label via i18n', () => {
    render(
      <MemoryRouter initialEntries={['/history/docs/getting-started']}>
        <ShortcutsDialog />
      </MemoryRouter>,
    )

    expect(screen.getByText('Current page mode: history')).toBeInTheDocument()
    expect(
      screen.getByText(
        'Dialog shortcuts are also included while this dialog is open.',
      ),
    ).toBeInTheDocument()
    expect(screen.getByText('Back to page')).toBeInTheDocument()
    expect(screen.getByText('Dialog default action')).toBeInTheDocument()
    expect(screen.queryByText('Save page')).not.toBeInTheDocument()
  })
})
