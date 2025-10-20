import { Link } from 'react-router-dom'

import { AnchorHTMLAttributes, ReactNode, MouseEvent } from 'react'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'

interface MarkdownLinkProps extends AnchorHTMLAttributes<HTMLAnchorElement> {
  href?: string
  children?: ReactNode
}

export function MarkdownLink({ href, children, ...props }: MarkdownLinkProps) {
  // dialogs and tree hooks are used to determine existence and open create dialog
  const openDialog = useDialogsStore((s) => s.openDialog)
  const getPageByPath = useTreeStore((s) => s.getPageByPath)

  if (href === undefined) {
    return <>{children}</>
  }

  const isInternal =
    href &&
    !href.startsWith('http') &&
    !href.startsWith('mailto:') &&
    !href.startsWith('#')

  // Normalize relative hrefs to absolute
  const normalizedHref = href.startsWith('/') ? href : '/' + href // turn "leafwiki/roadmap" into "/leafwiki/roadmap"

  if (isInternal) {
    // check if it is a asset link
    if (href.startsWith('assets/') || href.startsWith('/assets/')) {
      return (
        <a
          href={href}
          {...props}
          target="_blank"
          rel="noopener noreferrer"
          className="text-blue-600 no-underline hover:underline dark:text-blue-400"
        >
          {children}
        </a>
      )
    }

    // Determine the target path (strip leading /, '/e/' editor prefix, query and hash)
    let targetPath = ''
    try {
      const url = new URL(normalizedHref, window.location.origin)
      let p = url.pathname
      if (p.startsWith('/e/')) p = p.slice(3)
      if (p.startsWith('/')) p = p.slice(1)
      targetPath = p
    } catch {
      // fallback naive approach
      let p = normalizedHref
      if (p.startsWith('/e/')) p = p.slice(3)
      if (p.startsWith('/')) p = p.slice(1)
      const qIndex = p.indexOf('?')
      if (qIndex !== -1) p = p.slice(0, qIndex)
      const hashIndex = p.indexOf('#')
      if (hashIndex !== -1) p = p.slice(0, hashIndex)
      targetPath = p
    }

    const page = targetPath ? getPageByPath(targetPath) : null

    // If page does not exist locally, render red link and open AddPageDialog on click
    if (!page) {
      const parentPath = targetPath.includes('/')
        ? targetPath.substring(0, targetPath.lastIndexOf('/'))
        : ''
      const parentNode = parentPath ? getPageByPath(parentPath) : null
      const parentId = parentNode ? parentNode.id : ''

      const handleClick = (e: MouseEvent) => {
        e.preventDefault()
        openDialog('add', { parentId })
      }

      return (
        <a
          href={normalizedHref}
          {...props}
          onClick={handleClick}
          className="text-red-600 no-underline hover:underline dark:text-red-400"
          title="Page does not exist — click to create"
        >
          {children}
        </a>
      )
    }

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
    <a
      href={href}
      {...props}
      target="_blank"
      rel="noopener noreferrer"
      className="text-blue-600 no-underline hover:underline dark:text-blue-400"
    >
      {children}
    </a>
  )
}
