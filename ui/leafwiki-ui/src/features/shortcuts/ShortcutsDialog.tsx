import BaseDialog from '@/components/BaseDialog'
import i18next from '@/lib/i18n'
import { DIALOG_SHORTCUTS_HELP } from '@/lib/registries'

const shortcutRows = [
  {
    actionKey: 'shortcutsHelp.items.goToPage.action',
    keys: ['Cmd+Option+P', 'Ctrl+Alt+P'],
  },
  {
    actionKey: 'shortcutsHelp.items.openExplorer.action',
    keys: ['Mod+Shift+E'],
  },
  {
    actionKey: 'shortcutsHelp.items.openSearch.action',
    keys: ['Mod+Shift+F'],
  },
  {
    actionKey: 'shortcutsHelp.items.closeDialog.action',
    keys: ['Esc'],
  },
] as const

export function ShortcutsDialog() {
  return (
    <BaseDialog
      dialogType={DIALOG_SHORTCUTS_HELP}
      dialogTitle={i18next.t('shortcutsHelp.title', { ns: 'viewer' })}
      dialogDescription={i18next.t('shortcutsHelp.description', {
        ns: 'viewer',
      })}
      testidPrefix="shortcuts-help-dialog"
      cancelButton={{
        label: i18next.t('shortcutsHelp.closeButton', { ns: 'viewer' }),
        variant: 'outline',
        autoFocus: true,
      }}
      defaultAction="cancel"
      onClose={() => true}
      onConfirm={async () => true}
    >
      <div className="space-y-3 pt-2" data-testid="shortcuts-help-dialog">
        <div className="rounded-md border">
          <div className="grid grid-cols-[minmax(0,1fr)_auto] gap-x-4 gap-y-3 p-4">
            {shortcutRows.map((row) => (
              <div key={row.actionKey} className="contents">
                <span className="text-sm">
                  {i18next.t(row.actionKey, { ns: 'viewer' })}
                </span>
                <span className="text-muted-foreground text-right text-sm font-medium whitespace-nowrap">
                  {row.keys.join(' / ')}
                </span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </BaseDialog>
  )
}
