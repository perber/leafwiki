import BaseDialog from '@/components/BaseDialog'
import i18next from '@/lib/i18n'
import {
  getShortcutDisplayLabel,
  getVisibleShortcutsForMode,
} from '@/lib/shortcuts/shortcutCatalog'
import { useAppMode } from '@/lib/useAppMode'
import { DIALOG_SHORTCUTS_HELP } from '@/lib/registries'

const isMacOS =
  typeof navigator !== 'undefined' &&
  /Mac|iPhone|iPad|iPod/.test(navigator.platform)

export function ShortcutsDialog() {
  const appMode = useAppMode()
  const shortcuts = getVisibleShortcutsForMode(appMode)

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
        <p className="text-muted-foreground text-sm font-medium">
          {i18next.t('shortcutsHelp.currentMode', {
            ns: 'viewer',
            mode: i18next.t(`shortcutsHelp.modes.${appMode}`, { ns: 'viewer' }),
          })}
        </p>
        <div className="rounded-md border">
          <div className="grid grid-cols-[minmax(0,1fr)_auto] gap-x-4 gap-y-3 p-4">
            {shortcuts.map((shortcut) => (
              <div key={shortcut.id} className="contents">
                <span className="text-sm">
                  {i18next.t(shortcut.labelKey, { ns: 'viewer' })}
                </span>
                <span className="text-muted-foreground text-right text-sm font-medium whitespace-nowrap">
                  {getShortcutDisplayLabel(shortcut.id, isMacOS)}
                </span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </BaseDialog>
  )
}
