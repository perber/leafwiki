import { Fragment } from 'react'

type HighlightedSearchMarkupProps = {
  text: string
}

const highlightTagPattern = /(<\/?b>)/
const htmlEntityDecoder =
  typeof document === 'undefined' ? null : document.createElement('textarea')

function decodeHtmlEntities(value: string) {
  if (!value.includes('&') || htmlEntityDecoder == null) {
    return value
  }

  htmlEntityDecoder.innerHTML = value
  return htmlEntityDecoder.value
}

export default function HighlightedSearchTitle({
  text,
}: HighlightedSearchMarkupProps) {
  let insideHighlight = false

  return text.split(highlightTagPattern).map((part, index) => {
    if (part === '<b>') {
      insideHighlight = true
      return null
    }

    if (part === '</b>') {
      insideHighlight = false
      return null
    }

    if (part === '') {
      return null
    }

    const text = decodeHtmlEntities(part)

    if (insideHighlight) {
      return <b key={`title-part-${index}`}>{text}</b>
    }

    return <Fragment key={`title-part-${index}`}>{text}</Fragment>
  })
}
