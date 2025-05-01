import { EditorTitleBar } from '@/components/EditorTitleBar'
import MarkdownEditor from '@/components/MarkdownEditor'
import { TooltipWrapper } from '@/components/TooltipWrapper'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { UnsavedChangesDialog } from '@/components/UnsavedChangesDialog'
import { usePageToolbar } from '@/components/usePageToolbar'
import { getPageByPath, updatePage } from '@/lib/api'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { DialogDescription } from '@radix-ui/react-dialog'
import { Save, X } from 'lucide-react'
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { AssetManager } from './AssetManager'

export default function PageEditor() {
  const { '*': path } = useParams()
  interface Page {
    id: string
    path: string
    title: string
    slug: string
    content: string
  }

  const [page, setPage] = useState<Page | null>(null)
  const [inserted, setInserted] = useState<string | null>(null)
  const navigate = useNavigate()
  const [markdown, setMarkdown] = useState('')
  const [isNavigatingAway, setIsNavigatingAway] = useState(false)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [assetModalOpen, setAssetModalOpen] = useState(false)
  const reloadTree = useTreeStore((s) => s.reloadTree)
  const [, setFieldErrors] = useState<Record<string, string>>({})
  const [showUnsavedDialog, setShowUnsavedDialog] = useState(false)
  const [pendingNavigation, setPendingNavigation] = useState<string | null>(
    null,
  )

  const openDialog = useDialogsStore((state) => state.openDialog)
  const findPageInTreeByPath = useTreeStore((state) => state.getPageByPath)

  const [title, setTitle] = useState('')
  const [slug, setSlug] = useState('')

  const { setContent, clearContent, setTitleBar, clearTitleBar } =
    usePageToolbar()

  const handleSaveRef = useRef<() => void>(() => {})

  const onMetaDataChange = useCallback((title: string, slug: string) => {
    setTitle(title)
    setSlug(slug)
  }, [])

  const parentPath =
    useTreeStore(() => {
      const parts = page?.path?.split('/')
      parts?.pop()
      return parts?.join('/')
    }) || ''

  // useMemo to get parentId
  // This is a workaround to avoid running against the whole tree
  const parentId = useMemo(() => {
    const p = findPageInTreeByPath(parentPath)
    if (!p) return ''
    return p.id
  }, [parentPath, findPageInTreeByPath])

  // isDirty is a boolean that indicates if the page is dirty
  // We use it to identify if the user has unsaved changes
  // isDirty is true if the title, slug or markdown content has changed
  // and the user is not navigating away
  const isDirty = useMemo(() => {
    if (!page) return false
    return (
      title !== page.title || slug !== page.slug || markdown !== page.content
    )
  }, [page, title, slug, markdown])

  const shouldPromptUnsaved = !isNavigatingAway && isDirty

  // handeNavigateAway is a function that handles the navigation away from the page
  // It checks if the user has unsaved changes
  // If the user has unsaved changes, we show the UnsavedChangesDialog
  // If the user doesn't have unsaved changes, we navigate to the new page
  // We set the pendingNavigation state to the new path
  const handleNavigateAway = useCallback(
    (targetPath: string) => {
      if (pendingNavigation || showUnsavedDialog || isNavigatingAway) return

      if (shouldPromptUnsaved) {
        setPendingNavigation(targetPath)
        setShowUnsavedDialog(true)
      } else {
        navigate(targetPath)
      }
    },
    [
      shouldPromptUnsaved,
      navigate,
      pendingNavigation,
      showUnsavedDialog,
      isNavigatingAway,
    ],
  )

  useEffect(() => {
    handleSaveRef.current = async () => {
      if (!isDirty || !page) return
      try {
        toast.info('Saving page...')
        await updatePage(page.id, title, slug, markdown)
        toast.success('Page saved successfully!')
        // Set new page content after save
        setPage({
          ...page,
          title,
          slug,
          content: markdown,
        })

        // The slug might have changed, so we need to update the path
        const newPath = `/e${parentPath}/${slug}`
        // We don't want to redirect the user to the new path
        // because we are already on the page

        window.history.replaceState(null, '', newPath)
      } catch (err) {
        console.warn(err)
        handleFieldErrors(err, setFieldErrors, 'Error saving page')
      }
    }
  }, [page, title, slug, markdown, parentPath, isDirty])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 's') {
        e.preventDefault()
        handleSaveRef.current()
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [])

  // The users presses escape in the editor
  // If the user has unsaved changes, show the dialog
  // We only show the dialog if the user is not in the asset modal or the UnsavedChangesDialog
  useEffect(() => {
    const handleEscape = async (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return

      if (!showUnsavedDialog && !assetModalOpen) {
        await reloadTree()
        handleNavigateAway(parentPath ? `/${parentPath}/${slug}` : '/' + slug)
      }

      e.preventDefault()
    }

    window.addEventListener('keydown', handleEscape)
    return () => window.removeEventListener('keydown', handleEscape)
  }, [
    parentPath,
    slug,
    handleNavigateAway,
    reloadTree,
    showUnsavedDialog,
    assetModalOpen,
  ])

  // The user clicks the edit button in the title bar
  // We open the edit page metadata dialog
  // and pass the current title and slug to it
  // The dialog will call the onMetaDataChange function
  // when the user clicks save
  // This will set the new title and slug in the editor
  const onEditTitleClicked = useCallback(() => {
    openDialog('edit-page-metadata', {
      title,
      slug,
      parentId,
      onChange: onMetaDataChange,
    })
  }, [title, slug, parentId, onMetaDataChange, openDialog])

  // We set the title bar and content of the page editor
  useEffect(() => {
    if (!page || !title || !slug) return

    setTitleBar(
      <EditorTitleBar
        title={title}
        slug={slug}
        onEditClicked={onEditTitleClicked}
        isDirty={isDirty}
      />,
    )

    return () => {
      clearTitleBar()
    }
  }, [
    title,
    slug,
    parentId,
    openDialog,
    isDirty,
    clearTitleBar,
    onEditTitleClicked,
    page,
    setTitleBar,
  ])

  // We set the content of the page editor
  // This will be shown in the title bar
  useEffect(() => {
    if (!page) return
    setContent(
      <React.Fragment key="editing">
        <TooltipWrapper label="Close (ESC)" side="top" align="center">
          <Button
            variant="destructive"
            className="h-8 w-8 rounded-full shadow-sm"
            size="icon"
            onClick={async () => {
              await reloadTree()
              // When the user presses the close button
              // we want to navigate away from the page
              // handleNavigateAway will check if the user has unsaved changes and show the dialog
              handleNavigateAway(
                parentPath ? `/${parentPath}/${slug}` : '/' + slug,
              )
            }}
          >
            <X />
          </Button>
        </TooltipWrapper>
        <TooltipWrapper label="Save (Ctrl+S)" side="top" align="center">
          <Button
            onClick={() => handleSaveRef.current()}
            variant="default"
            className="h-8 w-8 rounded-full shadow-md"
            size="icon"
            disabled={!isDirty}
          >
            <Save />
          </Button>
        </TooltipWrapper>
      </React.Fragment>,
    )

    return () => {
      clearContent()
    }
  }, [
    page,
    path,
    parentPath,
    slug,
    setContent,
    isDirty,
    clearContent,
    handleNavigateAway,
    reloadTree,
  ])

  // We load the page by path
  useEffect(() => {
    if (!path) return

    setLoading(true)
    getPageByPath(path)
      .then((resp) => {
        const { content, title, slug } = resp as {
          content: string
          title: string
          slug: string
        }
        setPage(resp as Page)
        setMarkdown(content)
        setTitle(title)
        setSlug(slug)
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))
  }, [path])

  // We set the document title to the page title
  useEffect(() => {
    if (title) {
      document.title = `${title} - Edit Page – LeafWiki`
    } else {
      document.title = 'Edit Page – LeafWiki'
    }

    return () => {
      // optional zurücksetzen oder leer lassen
      document.title = 'LeafWiki'
    }
  }, [title])

  if (loading)
    return (
      <>
        <p className="text-sm text-gray-500">Loading...</p>
      </>
    )
  if (error)
    return (
      <>
        <p className="text-sm text-red-500">Error: {error}</p>
      </>
    )
  if (!page)
    return (
      <>
        <p className="text-sm text-gray-500">No page found</p>
      </>
    )

  return (
    <>
      <div className="flex h-[calc(100vh-180px)] gap-6">
        <div className="flex flex-1 flex-col gap-2">
          <div className="flex justify-end pb-2">
            <Button variant="outline" onClick={() => setAssetModalOpen(true)}>
              + Add Asset
            </Button>
          </div>
          <MarkdownEditor
            ref={textareaRef}
            value={markdown}
            onChange={(val) => {
              setMarkdown(val)

              if (inserted) setInserted(null) // reset after insert
            }}
            insert={inserted}
          />
        </div>
        <Dialog open={assetModalOpen} onOpenChange={setAssetModalOpen}>
          <DialogContent
            className="max-w-2xl"
            onKeyDown={(e) => {
              if (e.key === 'Escape') {
                e.stopPropagation()
                e.preventDefault()
              }
            }}
          >
            <DialogHeader>
              <DialogTitle>Add Asset</DialogTitle>
              <DialogDescription>
                Upload or select an asset to insert into the page.
              </DialogDescription>
            </DialogHeader>
            <AssetManager
              pageId={page.id}
              onInsert={(md) => {
                setInserted(md)
                setAssetModalOpen(false)
              }}
            />
          </DialogContent>
        </Dialog>
      </div>
      {!isNavigatingAway && (
        <UnsavedChangesDialog
          open={showUnsavedDialog}
          onCancel={() => {
            // Close the dialog and reset the pending navigation
            setShowUnsavedDialog(false)
            setPendingNavigation(null)
            // set focus back to the editor
            requestAnimationFrame(() => {
              const el = textareaRef.current
              if (el) {
                el.focus()
                el.classList.add('flash-border')
                setTimeout(() => el.classList.remove('flash-border'), 400)
              }
            })
          }}
          onConfirm={() => {
            // if the user confirms, and not outstanding navigation
            // we want to navigate to the new page
            if (pendingNavigation) {
              setIsNavigatingAway(true)
              setShowUnsavedDialog(false)
              const path = pendingNavigation
              // navigate to the new page
              requestAnimationFrame(() => {
                navigate(path)
                setPendingNavigation(null)
              })
            }
          }}
        />
      )}
    </>
  )
}
