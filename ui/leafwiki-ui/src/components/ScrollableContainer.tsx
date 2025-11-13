import { ReactNode } from 'react'

export default function ScrollableContainer({
  children,
  hidden,
}: {
  children: ReactNode
  hidden?: boolean
}) {
  return (
    <div
      className={`custom-scrollbar h-full w-full overflow-y-auto p-2 ${hidden ? 'hidden' : ''}`}
    >
      {children}
    </div>
  )
}
