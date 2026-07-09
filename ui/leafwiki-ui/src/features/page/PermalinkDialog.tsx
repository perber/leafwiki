import BaseDialog from '@/components/BaseDialog'
import { FormInput } from '@/components/FormInput'
import { Button } from '@/components/ui/button'
import type { Page } from '@/lib/api/pages'
import { DIALOG_PAGE_PERMALINK } from '@/lib/registries'
import { buildPermalinkPath, withBasePath } from '@/lib/routePath'
import copy from 'copy-to-clipboard'
import { Copy, ExternalLink } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

type PermalinkDialogProps = {
  page: Pick<Page, 'id' | 'slug' | 'title'>
}

export function PermalinkDialog({ page }: PermalinkDialogProps) {
  const { t } = useTranslation('page')
  const { t: tCommon } = useTranslation('common')
  const permalink = useMemo(() => {
    const path = withBasePath(buildPermalinkPath(page.id, page.slug))
    if (typeof window === 'undefined') {
      return path
    }
    return new URL(path, window.location.origin).toString()
  }, [page.id, page.slug])

  const handleCopy = () => {
    if (!copy(permalink)) {
      toast.error(t('toast.permalinkCopyFailed'))
      return
    }

    toast.success(t('toast.permalinkCopied'))
  }

  const handleClose = () => true

  return (
    <BaseDialog
      dialogTitle={t('share.title')}
      dialogDescription={t('share.description', { title: page.title })}
      dialogType={DIALOG_PAGE_PERMALINK}
      onClose={handleClose}
      onConfirm={async () => false}
      testidPrefix="permalink-dialog"
      cancelButton={{
        label: tCommon('actions.close'),
        variant: 'outline',
      }}
    >
      <div className="space-y-3">
        <FormInput
          label={t('share.urlLabel')}
          value={permalink}
          onChange={() => {}}
          readOnly={true}
          testid="permalink-dialog-url-input"
          autoFocus={true}
        />
        <div className="flex items-center justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            onClick={handleCopy}
            data-testid="permalink-dialog-copy-button"
          >
            <Copy />
            {tCommon('actions.copyLink')}
          </Button>
          <Button
            type="button"
            variant="outline"
            asChild
            data-testid="permalink-dialog-open-button"
          >
            <a href={permalink} target="_blank" rel="noreferrer">
              <ExternalLink />
              {tCommon('actions.openLink')}
            </a>
          </Button>
        </div>
      </div>
    </BaseDialog>
  )
}

export default PermalinkDialog
