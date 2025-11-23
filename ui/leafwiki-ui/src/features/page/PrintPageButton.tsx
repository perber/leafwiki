import { PageToolbarButton } from '@/components/PageToolbarButton'
import { Printer } from 'lucide-react'

export function PrintPageButton() {
  return (
    <PageToolbarButton
      label="Print page"
      hotkey="Ctrl + P"
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
      icon={<Printer size={20} />}
    />
  )
}
