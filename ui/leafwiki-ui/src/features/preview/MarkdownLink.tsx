import { Link } from 'react-router'

import { Button } from '@/components/ui/button'
import { TooltipWrapper } from '@/components/TooltipWrapper'
import { createNavigationVisitState } from '@/lib/navigationVisit'
import {
  DIALOG_CREATE_PAGE_BY_PATH,
  DIALOG_WIKILINK_DISAMBIGUATION,
} from '@/lib/registries'
import { buildViewUrl, stripBasePath, withBasePath } from '@/lib/routePath'
import {
  normalizeWikiRoutePath,
  resolveWikiLinkPath,
  toWikiLookupPath,
} from '@/lib/wikiPath'
import { suggestSlug } from '@/lib/api/pages'
import { useAppMode } from '@/lib/useAppMode'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { AnchorHTMLAttributes, ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

function BrokenLinkTooltip({
  children,
  label,
}: {
  children: ReactNode
  label: string
}) {
  return (
    <TooltipWrapper label={label}>
      <span className="text-error cursor-default">{children}</span>
    </TooltipWrapper>
  )
}

function safeDecodeURIComponent(s: string): string {
  try {
    return decodeURIComponent(s)
  } catch {
    return s
  }
}

function isWebHref(href: string): boolean {
  return /^(https?:|\/\/)/i.test(href)
}

function isInternalHref(href: string): boolean {
  return (
    href !== '' &&
    !isWebHref(href) &&
    !href.startsWith('http') &&
    !href.startsWith('mailto:') &&
    !href.startsWith('#')
  )
}

function isAssetHref(href: string): boolean {
  return href.startsWith('assets/') || href.startsWith('/assets/')
}

interface MarkdownLinkProps extends AnchorHTMLAttributes<HTMLAnchorElement> {
  href?: string
  children?: ReactNode
  path?: string
  node?: unknown
  resolveAssetUrl?: (src: string) => string
}

export function MarkdownLink({
  href,
  children,
  node,
  resolveAssetUrl,
  ...props
}: MarkdownLinkProps) {
  void node
  const { t } = useTranslation('viewer')
  const openDialog = useDialogsStore((s) => s.openDialog)
  const getPageByPath = useTreeStore((s) => s.getPageByPath)
  const getPageById = useTreeStore((s) => s.getPageById)

  const editMode = useAppMode() === 'edit'
  const readOnly = useIsReadOnly()

  const getCurrentWikiPath = (): string => {
    let locationPath = window.location.pathname
    const stripped = stripBasePath(locationPath)
    if (stripped !== null) {
      locationPath = stripped
    }
    return normalizeWikiRoutePath(props.path ?? buildViewUrl(locationPath))
  }

  // Resolve the wiki target before any early return so the tree subscription
  // below is unconditional. A stable lookup function such as `getPageByPath`
  // never triggers a re-render when the tree arrives or changes, so the link
  // would keep its stale resolved/broken state.
  let normalizedHref: string | null = null
  if (
    href !== undefined &&
    !href.startsWith('wikilink-ambiguous:') &&
    !href.startsWith('wikilink-notfound:') &&
    isInternalHref(href) &&
    !isAssetHref(href)
  ) {
    // Absolute (e.g. "/stoff/change") or relative (e.g. "../stoff/change",
    // "child-page", "./foo") to the current page.
    normalizedHref = href.startsWith('/')
      ? normalizeWikiRoutePath(href)
      : resolveWikiLinkPath(getCurrentWikiPath(), href)
  }
  // lookupPath is the path without leading /, without query and hash
  const lookupPath =
    normalizedHref === null ? null : toWikiLookupPath(normalizedHref)
  const targetPage = useTreeStore((s) =>
    lookupPath === null ? null : (s.byPath[lookupPath] ?? null),
  )

  if (href === undefined) {
    return <>{children}</>
  }

  if (href.startsWith('wikilink-ambiguous:')) {
    const title = safeDecodeURIComponent(
      href.slice('wikilink-ambiguous:'.length),
    )
    return (
      <Button
        variant="link"
        onClick={() => openDialog(DIALOG_WIKILINK_DISAMBIGUATION, { title })}
        className="text-brand hover:text-brand-dark m-0 p-0 text-base no-underline hover:no-underline"
      >
        {children}
      </Button>
    )
  }

  if (href.startsWith('wikilink-notfound:')) {
    const title = safeDecodeURIComponent(
      href.slice('wikilink-notfound:'.length),
    )
    if (!readOnly) {
      return (
        <Button
          variant="link"
          onClick={async () => {
            // Create the new page in the same folder as the page containing
            // the link, not at the wiki root.
            const currentPage = getPageByPath(
              toWikiLookupPath(getCurrentWikiPath()),
            )
            const parentId = currentPage?.parentId ?? ''
            const folderPath = parentId
              ? (getPageById(parentId)?.path ?? '')
              : ''

            // Ask the server for a valid slug so the create-page dialog
            // receives a proper path instead of the raw title (which may
            // contain spaces or special characters).
            const slug = await suggestSlug(parentId, title).catch(() => '')
            const initialPath = [folderPath, slug || title]
              .filter(Boolean)
              .join('/')
            openDialog(DIALOG_CREATE_PAGE_BY_PATH, {
              initialPath,
              initialTitle: title,
              forwardToEditMode: !editMode,
            })
          }}
          className="text-error hover:text-error/80 m-0 p-0 text-base no-underline hover:no-underline"
        >
          {children}
        </Button>
      )
    }
    return (
      <BrokenLinkTooltip label={t('markdownPreview.brokenLinkTooltip')}>
        {children}
      </BrokenLinkTooltip>
    )
  }

  const isWebLink = isWebHref(href)
  let isSameOrigin = false
  if (isWebLink) {
    try {
      const url = new URL(href, window.location.href)
      isSameOrigin =
        (url.protocol === 'http:' || url.protocol === 'https:') &&
        url.origin === window.location.origin
    } catch {
      // Malformed web URLs must not enter wiki path resolution.
    }
  }

  const handleOpenCreatePageDialog = (path: string) => {
    openDialog(DIALOG_CREATE_PAGE_BY_PATH, {
      initialPath: path,
      readOnlyPath: true,
      forwardToEditMode: !editMode,
    })
  }

  if (isInternalHref(href) && isAssetHref(href)) {
    const path = href.startsWith('/assets/')
      ? href
      : '/assets/' + href.slice('assets/'.length)

    const resolvedPath = resolveAssetUrl?.(path) ?? path
    const assetHref = withBasePath(resolvedPath)
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

  if (normalizedHref !== null && lookupPath !== null) {
    /**
     *  When a page link is internal and not an asset link and the page doesn't exist yet,
     * we will color the link in red and offer to create the page. Via the CreatePageByPathDialog.
     * we should handle and calculate relative paths here as well.
     * normalizedHref contains now the absolute path. We can use it directly.
     **/
    const normalizedTargetPath = lookupPath
    const pageExists = !!targetPage
    if (!pageExists && !readOnly) {
      return (
        <Button
          variant="link"
          onClick={() => {
            handleOpenCreatePageDialog(normalizedTargetPath)
          }}
          className="text-error hover:text-error/80 m-0 p-0 text-base no-underline hover:no-underline"
        >
          {children}
        </Button>
      )
    }

    if (!pageExists) {
      return (
        <BrokenLinkTooltip label={t('markdownPreview.brokenLinkTooltip')}>
          {children}
        </BrokenLinkTooltip>
      )
    }

    return (
      <Link
        to={normalizedHref}
        state={createNavigationVisitState()}
        {...props}
        className="no-underline hover:underline"
      >
        {children}
      </Link>
    )
  }

  return (
    <a
      href={href}
      {...props}
      target={isSameOrigin || href.startsWith('#') ? undefined : '_blank'}
      rel="noopener noreferrer"
      className="text-brand hover:text-brand-dark no-underline hover:underline"
    >
      {children}
    </a>
  )
}
