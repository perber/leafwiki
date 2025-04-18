import { Link } from 'react-router-dom'

export function MarkdownLink({ href, children, ...props }: any) {
  const isInternal =
    href &&
    !href.startsWith('http') &&
    !href.startsWith('mailto:') &&
    !href.startsWith('#')

  // Normalize relative hrefs to absolute
  const normalizedHref = href.startsWith('/') ? href : '/' + href // turn "leafwiki/roadmap" into "/leafwiki/roadmap"

  if (isInternal) {
    return (
      <Link
        to={normalizedHref}
        {...props}
        className="text-blue-600 no-underline hover:underline dark:text-blue-400"
      >
        {children}
      </Link>
    )
  }

  return (
    <a href={href} {...props} target="_blank" rel="noopener noreferrer">
      {children}
    </a>
  )
}
