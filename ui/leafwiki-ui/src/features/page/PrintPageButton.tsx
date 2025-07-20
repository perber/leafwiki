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
        onClick={() => {
          if (typeof window.print === 'function') {
            try {
              window.print()
            } catch (error) {
              console.error('Failed to print the page:', error)
            }
          } else {
            console.error('Printing is not supported in this environment.')
          }
        }}
      >
        <Printer size={20} />
      </Button>
    </TooltipWrapper>
  )
}
