type TreeViewActionButtonProps = {
  onClick: () => void
  icon: React.ReactNode
  tooltip: string
}

export function TreeViewActionButton({
  onClick,
  icon,
  tooltip,
}: TreeViewActionButtonProps) {
  return (
    <div className="group relative mr-2 flex">
      <button onClick={() => onClick()}>{icon}</button>
      <div className="absolute bottom-full left-0 mb-2 hidden w-max rounded bg-gray-700 px-2 py-1 text-xs text-white group-hover:block">
        {tooltip}
      </div>
    </div>
  )
}
