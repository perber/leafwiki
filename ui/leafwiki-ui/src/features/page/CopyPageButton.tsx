import { TooltipWrapper } from '@/components/TooltipWrapper'
import { Button } from '@/components/ui/button'
import { Page } from '@/lib/api/pages'
import { useDialogsStore } from '@/stores/dialogs'
import { Copy } from 'lucide-react'

export function CopyPageButton({
  sourcePage,
}: {
  sourcePage: Page
}) {
  const openDialog = useDialogsStore((s) => s.openDialog)

  return (
    <TooltipWrapper label="Copy page" side="top" align="center">
      <Button
        className="h-8 w-8 rounded-full shadow-xs"
        variant="default"
        size="icon"
        onClick={() => {
          openDialog('copy-page', { sourcePage })
        }}
      >
        <Copy size={20} />
      </Button>
    </TooltipWrapper>
  )
}
