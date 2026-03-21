import { Link } from 'react-router-dom'

import { Button } from '@/components/ui/button'
import { DIALOG_CREATE_PAGE_BY_PATH } from '@/lib/registries'
import { buildViewUrl, stripBasePath, withBasePath } from '@/lib/routePath'
import {
  normalizeWikiRoutePath,
  resolveWikiLinkPath,
  toWikiLookupPath,
} from '@/lib/wikiPath'
import { useAppMode } from '@/lib/useAppMode'
import { useDialogsStore } from '@/stores/dialogs'
import { useSessionStore } from '@/stores/session'
import { useTreeStore } from '@/stores/tree'
import clsx from 'clsx'
import { AnchorHTMLAttributes, ReactNode } from 'react'

interface MarkdownLinkProps extends AnchorHTMLAttributes<HTMLAnchorElement> {
  href?: string
  children?: ReactNode
  path?: string
}

export function MarkdownLink({ href, children, ...props }: MarkdownLinkProps) {
  const openDialog = useDialogsStore((s) => s.openDialog)
  const getPageByPath = useTreeStore((s) => s.getPageByPath)
  const user = useSessionStore((s) => s.user)

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
      const path = href.startsWith('/assets/')
        ? href
        : '/assets/' + href.slice('assets/'.length)

      const assetHref = withBasePath(path)
      return (
        <a
          href={assetHref}
          {...props}
          target="_blank"
          rel="noopener noreferrer"
          className="text-brand hover:text-brand-dark no-underline hover:underline"
        >
          {children}
        </a>
      )
    }
    /*
      First we need to check if it is a relative link or an absolute link.
    */
    let normalizedHref = href
    if (href.startsWith('/')) {
      // Already absolute (e.g. "/stoff/change")
      normalizedHref = normalizeWikiRoutePath(href)
    } else {
      // Relative link (e.g. "../stoff/change", "child-page", "./foo")
      let locationPath = window.location.pathname

      // Use stripBasePath utility (with boundary check)
      const stripped = stripBasePath(locationPath)
      if (stripped !== null) {
        locationPath = stripped
      }

      // Then proceed as before
      const currentPath = normalizeWikiRoutePath(
        props.path ?? buildViewUrl(locationPath),
      )

      normalizedHref = resolveWikiLinkPath(currentPath, href)
    }

    /**
     *  When a page link is internal and not an asset link and the page doesn't exist yet,
     * we will color the link in red and offer to create the page. Via the CreatePageByPathDialog.
     * we should handle and calculate relative paths here as well.
     * normalizedHref contains now the absolute path. We can use it directly.
     **/

    // normalizedTargetPath is the path without leading /, without query and hash
    const normalizedTargetPath = toWikiLookupPath(normalizedHref)

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
          className="text-error hover:text-error/80 m-0 p-0 text-base no-underline hover:no-underline"
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
          'no-underline hover:underline',
          !user && !pageExists && 'text-error',
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
      target={href.startsWith('#') ? undefined : '_blank'}
      rel="noopener noreferrer"
      className="text-brand hover:text-brand-dark no-underline hover:underline"
    >
      {children}
    </a>
  )
}
