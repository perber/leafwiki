import { Progress } from '@/components/ui/progress'
import { useEffect, useRef, useState } from 'react'
import { useProgressbarStore } from './progressbar'

type TimeoutId = ReturnType<typeof setTimeout>
type IntervalId = ReturnType<typeof setInterval>

export default function Progressbar() {
  const loading = useProgressbarStore((state) => state.loading)

  const [value, setValue] = useState<number>(0)
  const [showLoadingbar, setShowLoadingbar] = useState<boolean>(false)

  const showTimerRef = useRef<TimeoutId | null>(null)
  const hideTimerRef = useRef<TimeoutId | null>(null)
  const intervalRef = useRef<IntervalId | null>(null)

  // Helper functions for cleanup
  const clearShowTimer = () => {
    if (showTimerRef.current) {
      clearTimeout(showTimerRef.current)
      showTimerRef.current = null
    }
  }

  const clearHideTimer = () => {
    if (hideTimerRef.current) {
      clearTimeout(hideTimerRef.current)
      hideTimerRef.current = null
    }
  }

  const clearIntervalTimer = () => {
    if (intervalRef.current) {
      clearInterval(intervalRef.current)
      intervalRef.current = null
    }
  }

  // Controls visibility + delay
  useEffect(() => {
    if (loading) {
      // if a hide was planned -> cancel it
      clearHideTimer()

      // Reset progress & start show delay
      clearShowTimer()
      setValue(0)

      showTimerRef.current = setTimeout(() => {
        setValue(20) // start at 20%
        setShowLoadingbar(true)
      }, 100)
    } else {
      // Loading finished: stop show delay
      clearShowTimer()

      if (showLoadingbar) {
        // briefly go to 100% and then hide
        setValue(100)

        clearHideTimer()
        hideTimerRef.current = setTimeout(() => {
          setShowLoadingbar(false)
          setValue(0)
        }, 150)
      } else {
        // if the bar was never visible: just clean everything up
        clearHideTimer()
        setShowLoadingbar(false)
        setValue(0)
      }
    }

    return () => {
      clearShowTimer()
      clearHideTimer()
      clearIntervalTimer()
    }
    // we want to run this effect only when 'loading' changes
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loading])

  // fills the progress bar gradually
  useEffect(() => {
    clearIntervalTimer()

    if (showLoadingbar) {
      intervalRef.current = setInterval(() => {
        setValue((prev) => {
          if (prev < 90) {
            const next = prev + Math.random() * 10
            return next > 90 ? 90 : next
          }
          return prev
        })
      }, 200)
    }

    return () => {
      clearIntervalTimer()
    }
  }, [showLoadingbar])

  if (!showLoadingbar) return null

  return (
    <Progress
      value={value}
      className="[&>div]:bg-brand-dark absolute top-0 left-0 z-50 h-1 rounded-none [&>div]:h-1 [&>div]:rounded-none"
    />
  )
}
