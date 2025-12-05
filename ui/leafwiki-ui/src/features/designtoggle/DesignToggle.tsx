import { Moon, Sun } from 'lucide-react'
import { Button } from '../../components/ui/button'
import { useDesignModeStore } from './designmode'

export default function DesignToggle() {
  const mode = useDesignModeStore((s) => s.mode)
  const setMode = useDesignModeStore((s) => s.setMode)

  return (
    <Button
      variant="ghost"
      size="icon"
      aria-label={
        mode === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'
      }
      onClick={() => setMode(mode === 'dark' ? 'light' : 'dark')}
    >
      <Moon
        className={mode === 'light' || mode === 'system' ? 'visible' : 'hidden'}
      />
      <Sun className={mode === 'dark' ? 'visible' : 'hidden'} />
    </Button>
  )
}
