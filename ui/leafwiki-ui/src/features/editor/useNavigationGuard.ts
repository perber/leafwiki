import { useEffect, useState } from 'react'
import { useBlocker } from 'react-router-dom'

export function useNavigationGuard(shouldBlock: boolean) {
  const blocker = useBlocker(() => shouldBlock)
  const [showDialog, setShowDialog] = useState(false)

  useEffect(() => {
    if (blocker.state === 'blocked') {
      setShowDialog(true)
    }
  }, [blocker.state])

  const onConfirm = () => {
    setShowDialog(false)
    if (blocker.proceed) {
      blocker.proceed()
    }
  }

  const onCancel = () => {
    setShowDialog(false)
    if (blocker.proceed) {
      blocker.reset()
    }
  }

  return { showDialog, onConfirm, onCancel }
}
