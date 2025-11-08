/* eslint-disable react-hooks/set-state-in-effect */
import { useEffect, useState } from 'react'

type Props = React.ImgHTMLAttributes<HTMLImageElement>

export function MarkdownImage({ src = '', alt, ...rest }: Props) {
  const [versionedSrc, setVersionedSrc] = useState(src)

  useEffect(() => {
    if (!src?.startsWith('/assets/')) {
      setVersionedSrc(src)
      return
    }

    const checkVersion = async () => {
      try {
        const v = Date.now()
        const url = new URL(src, location.origin)
        url.searchParams.set('v', v.toString())
        setVersionedSrc(url.toString())
      } catch {
        setVersionedSrc(src) // fallback
      }
    }

    checkVersion()
  }, [src])

  return <img src={versionedSrc} alt={alt} {...rest} />
}
