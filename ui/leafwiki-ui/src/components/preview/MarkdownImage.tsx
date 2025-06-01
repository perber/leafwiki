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
        setVersionedSrc(`${src}?v=${v}`)
      } catch {
        setVersionedSrc(src) // fallback
      }
    }

    checkVersion()
  }, [src])

  return <img src={versionedSrc} alt={alt} {...rest} />
}
