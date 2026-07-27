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
  const permalink = useMemo(() => {
    const path = withBasePath(buildPermalinkPath(page.id, page.slug))
    if (typeof window === 'undefined') {
      return path
    }
    return new URL(path, window.location.origin).toString()
  }, [page.id, page.slug])

  const handleCopy = () => {
    if (!copy(permalink)) {
      toast.error(t('permalinkDialog.copyErrorToast'))
      return
    }

    toast.success(t('permalinkDialog.copiedToast'))
  }

  const handleClose = () => true

  return (
    <BaseDialog
      dialogTitle={t('permalinkDialog.title')}
      dialogDescription={t('permalinkDialog.description', {
        title: page.title,
      })}
      dialogType={DIALOG_PAGE_PERMALINK}
      onClose={handleClose}
      onConfirm={async () => false}
      testidPrefix="permalink-dialog"
      cancelButton={{
        label: t('permalinkDialog.close'),
        variant: 'outline',
      }}
    >
      <div className="space-y-3">
        <FormInput
          label={t('permalinkDialog.urlLabel')}
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
            {t('permalinkDialog.copyLink')}
          </Button>
          <Button
            type="button"
            variant="outline"
            asChild
            data-testid="permalink-dialog-open-button"
          >
            <a href={permalink} target="_blank" rel="noreferrer">
              <ExternalLink />
              {t('permalinkDialog.openLink')}
            </a>
          </Button>
        </div>
      </div>
    </BaseDialog>
  )
}

export default PermalinkDialog
