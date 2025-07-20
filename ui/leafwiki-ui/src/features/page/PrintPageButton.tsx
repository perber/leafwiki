import { TooltipWrapper } from '@/components/TooltipWrapper'
import { Button } from '@/components/ui/button'
import { Printer } from 'lucide-react'

export function PrintPageButton() {
  return (
    <TooltipWrapper label="Print page (Ctrl + p)" side="top" align="center">
      <Button
        className="h-8 w-8 rounded-full shadow-xs"
        variant="default"
        size="icon"
        onClick={() => window.print()}
      >
        <Printer size={20} />
      </Button>
    </TooltipWrapper>
  )
}
