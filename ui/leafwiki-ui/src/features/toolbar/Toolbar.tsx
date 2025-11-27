import { ToolbarButton } from '@/features/toolbar/ToolbarButton'
import { useToolbarStore } from './toolbar'

export function Toolbar() {
  const buttons = useToolbarStore((state) => state.buttons)

  return (
    <>
      {buttons.map((button) => (
        <ToolbarButton
          key={button.id}
          hotkey={button.hotkey}
          label={button.label}
          onClick={button.action}
          icon={button.icon}
          disabled={button.disabled}
          variant={button.variant}
          className={button.className}
        />
      ))}
    </>
  )
}
