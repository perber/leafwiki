// /lib/useMeasure.ts
import { useEffect, useRef, useState } from 'react';

export function useMeasure<T extends HTMLElement>() {
  const ref = useRef<T>(null);
  const [height, setHeight] = useState(0);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;

    const observer = new ResizeObserver(([entry]) => {
      const newHeight = entry.contentRect.height;
      setHeight(newHeight);
    });

    observer.observe(el);

    // Initialhöhe setzen (für statischen Inhalt)
    setHeight(el.scrollHeight);

    return () => observer.disconnect();
  }, []);

  return [ref, height] as const;
}
