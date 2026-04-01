import { Moon, Sun } from 'lucide-react'
import { Button } from '../../components/ui/button'
import { useDesignModeStore } from './designmode'
import { TooltipWrapper } from '@/components/TooltipWrapper'

export default function DesignToggle() {
  const mode = useDesignModeStore((s) => s.mode)
  const setMode = useDesignModeStore((s) => s.setMode)
  const label = mode === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'

  return (
    <TooltipWrapper label={label} align="center">
      <Button
        variant="outline"
        size="icon"
        aria-label={label}
        onClick={() => {
          const effectiveMode =
            mode === 'system'
              ? window.matchMedia &&
                window.matchMedia('(prefers-color-scheme: dark)').matches
                ? 'dark'
                : 'light'
              : mode
          setMode(effectiveMode === 'dark' ? 'light' : 'dark')
        }}
      >
        <Moon
          className={mode === 'light' || mode === 'system' ? 'visible' : 'hidden'}
        />
        <Sun className={mode === 'dark' ? 'visible' : 'hidden'} />
      </Button>
    </TooltipWrapper>
  )
}
