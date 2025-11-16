import { Link } from 'react-router-dom'

import { Button } from '@/components/ui/button'
import { DIALOG_CREATE_PAGE_BY_PATH } from '@/lib/registries'
import { buildViewUrl } from '@/lib/urlUtil'
import { useAppMode } from '@/lib/useAppMode'
import { useAuthStore } from '@/stores/auth'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import clsx from 'clsx'
import { AnchorHTMLAttributes, ReactNode } from 'react'

interface MarkdownLinkProps extends AnchorHTMLAttributes<HTMLAnchorElement> {
  href?: string
  children?: ReactNode
}

export function MarkdownLink({ href, children, ...props }: MarkdownLinkProps) {
  const openDialog = useDialogsStore((s) => s.openDialog)
  const getPageByPath = useTreeStore((s) => s.getPageByPath)
  const user = useAuthStore((s) => s.user)

  const editMode = useAppMode() === 'edit'

  if (href === undefined) {
    return <>{children}</>
  }

  const isInternal =
    href &&
    !href.startsWith('http') &&
    !href.startsWith('mailto:') &&
    !href.startsWith('#')

  const handleOpenCreatePageDialog = (path: string, editMode: boolean) => {
    openDialog(DIALOG_CREATE_PAGE_BY_PATH, {
      initialPath: path,
      readOnlyPath: true,
      forwardToEditMode: !editMode,
    })
  }

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

    /*
      First we need to check if it is a relative link or an absolute link.
    */
    const absoluteHref = href.startsWith('/')
    let normalizedHref = href
    if (!absoluteHref) {
      // For relative links, we need to add the current path as prefix.
      let currentPath = buildViewUrl(window.location.pathname)
      // remove leading / to make path relative
      if (currentPath.startsWith('/')) {
        currentPath = currentPath.slice(1)
      }

      const basePath = currentPath
      // When the path contains ../ or ./ we need to resolve it
      const segments = href.split('/')
      const basePathSegments = basePath.split('/')
      for (const segment of segments) {
        if (segment === '..') {
          basePathSegments.pop()
        } else if (segment !== '.') {
          basePathSegments.push(segment)
        }
      }
      const resolvedPath = basePathSegments.join('/')

      // We calculate it to an absolute path
      normalizedHref = resolvedPath.startsWith('/')
        ? resolvedPath
        : '/' + resolvedPath
    }

    // When a page link is internal and not an asset link and the page doesn't exist yet,
    // we will color the link in red and offer to create the page. Via the CreatePageByPathDialog.
    // We should handle and calculate relative paths here as well.
    // normalizedHref contains now the absolute path. We can use it directly.

    // normalizedTargetPath is the path without leading /, without query and hash
    const normalizedTargetPath = normalizedHref
      .split('?')[0]
      .split('#')[0]
      .replace(/^\/+/, '')

    // Check if the page exists
    const page = getPageByPath(normalizedTargetPath)
    const pageExists = !!page
    if (!pageExists && user) {
      return (
        <Button
          variant="link"
          onClick={() => {
            handleOpenCreatePageDialog(normalizedTargetPath, editMode)
          }}
          className="m-0 p-0 text-base text-red-600 no-underline hover:no-underline dark:text-red-400"
        >
          {children}
        </Button>
      )
    }

    return (
      <Link
        to={normalizedHref}
        {...props}
        className={clsx(
          'no-underline hover:underline dark:text-blue-400',
          !user && !pageExists && '!text-red-600',
        )}
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
