import { TooltipWrapper } from '@/components/TooltipWrapper'
import { Button } from '@/components/ui/button'
import { DIALOG_DELETE_PAGE_CONFIRMATION } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { Trash2 } from 'lucide-react'

type DeletePageButtonProps = {
  pageId: string
  redirectUrl: string
}
export function DeletePageButton({
  pageId,
  redirectUrl,
}: DeletePageButtonProps) {
  const openDialog = useDialogsStore((s) => s.openDialog)

  return (
    <TooltipWrapper label="Delete page" side="top" align="center">
      <Button
        className="h-8 w-8 rounded-full shadow-xs"
        variant="destructive"
        size="icon"
        data-testid="delete-page-button"
        onClick={() => {
          openDialog(DIALOG_DELETE_PAGE_CONFIRMATION, { pageId, redirectUrl })
        }}
      >
        <Trash2 />
      </Button>
    </TooltipWrapper>
  )
}
