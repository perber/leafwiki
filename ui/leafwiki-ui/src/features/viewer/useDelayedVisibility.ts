import { useEffect, useRef, useState } from 'react'

type Options = {
  delay?: number          // ab wann überhaupt sichtbar
  minVisible?: number     // Mindest-Anzeigezeit
}

/**
 * Steuerung für "dumme Loader/Skeletons":
 * - Wird `active` schnell wieder false -> nie sichtbar
 * - Wird `active` länger true       -> nach `delay` sichtbar
 * - Bleibt danach mind. `minVisible` sichtbar
 */
export function useDelayedVisibility(
  active: boolean,
  { delay = 150, minVisible = 150 }: Options = {}
) {
  const [visible, setVisible] = useState(false)

  const delayTimerRef = useRef<number | null>(null)
  const hideTimerRef = useRef<number | null>(null)
  const visibleSinceRef = useRef<number | null>(null)

  useEffect(() => {
    const clearTimers = () => {
      if (delayTimerRef.current != null) {
        window.clearTimeout(delayTimerRef.current)
        delayTimerRef.current = null
      }
      if (hideTimerRef.current != null) {
        window.clearTimeout(hideTimerRef.current)
        hideTimerRef.current = null
      }
    }

    if (active) {
      // neuer Ladevorgang
      clearTimers()
      setVisible(false)
      visibleSinceRef.current = null

      // Skeleton erst nach delay anzeigen
      delayTimerRef.current = window.setTimeout(() => {
        setVisible(true)
        visibleSinceRef.current = Date.now()
        delayTimerRef.current = null
      }, delay)
    } else {
      // Laden fertig
      if (delayTimerRef.current != null) {
        // war noch in der Delay-Phase -> nie anzeigen
        clearTimers()
        setVisible(false)
        visibleSinceRef.current = null
      } else if (visibleSinceRef.current != null) {
        // Skeleton ist sichtbar -> Mindestzeit beachten
        const elapsed = Date.now() - visibleSinceRef.current

        if (elapsed >= minVisible) {
          clearTimers()
          setVisible(false)
          visibleSinceRef.current = null
        } else {
          const remaining = minVisible - elapsed
          hideTimerRef.current = window.setTimeout(() => {
            setVisible(false)
            visibleSinceRef.current = null
            clearTimers()
          }, remaining)
        }
      }
    }

    return () => {
      clearTimers()
    }
  }, [active, delay, minVisible])

  return visible
}