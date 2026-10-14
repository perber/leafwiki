import { uploadAsset, UploadAssetResponse } from '@/lib/api/assets'
import { mapApiError } from '@/lib/api/errors'
import { formatBytes, IMAGE_EXTENSIONS, PDF_EXTENSIONS } from '@/lib/config'
import { useConfigStore } from '@/stores/config'
import { EditorView } from '@codemirror/view'
import { ClipboardEvent, DragEvent, RefObject, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

type Options = {
  editorViewRef: RefObject<EditorView | null>
  pageId: string
  // Called after an upload's markdown reference was inserted into the view.
  onDocChange: (view: EditorView) => void
}

// useEditorFileUpload uploads files pasted or dropped onto the editor as page
// assets and inserts a markdown reference to each.
export function useEditorFileUpload({
  editorViewRef,
  pageId,
  onDocChange,
}: Options) {
  const { t } = useTranslation('editor')
  const maxAssetUploadSizeBytes = useConfigStore(
    (s) => s.maxAssetUploadSizeBytes,
  )

  // Uploads a single file and inserts its markdown reference into the editor.
  // Shared by paste-of-files and the drag-and-drop handler below. `insertPos`
  // pins the insertion to a specific document offset (the drop point); when
  // omitted, the cursor position at the time the upload resolves is used
  // instead, which is what paste wants.
  const uploadFileToEditor = useCallback(
    async (file: File, insertPos?: number) => {
      if (file.size > maxAssetUploadSizeBytes) {
        toast.error(
          t('markdownEditor.fileTooLarge', {
            maxSize: formatBytes(maxAssetUploadSizeBytes),
          }),
        )
        return
      }

      try {
        const res: UploadAssetResponse = await uploadAsset(pageId, file)

        toast.success(
          t('markdownEditor.uploadedToast', { filename: file.name }),
        )

        // The result of uploadAsset looks like this:
        // {"file":"/assets/0NmpvSivg/preview-scrollbar.gif"}
        const uploadedFile = res.file
        const ext = file.name.split('.').pop()?.toLowerCase()

        const isImage =
          file.type.startsWith('image/') || IMAGE_EXTENSIONS.includes(ext ?? '')
        const isPdf =
          file.type === 'application/pdf' || PDF_EXTENSIONS.includes(ext ?? '')

        const markdown =
          isImage || isPdf
            ? `![${file.name}](${uploadedFile})\n`
            : `[${file.name}](${uploadedFile})\n`

        const view = editorViewRef.current
        if (!view) return
        // The document may have changed while the upload was in flight;
        // clamp a caller-supplied drop position to the current doc length
        // so it can't land out of range.
        const from =
          insertPos !== undefined
            ? Math.min(insertPos, view.state.doc.length)
            : view.state.selection.main.from
        view.dispatch({
          changes: { from, insert: markdown },
          selection: { anchor: from + markdown.length },
        })

        onDocChange(view)
        editorViewRef.current?.focus()
      } catch (err) {
        console.error('Upload failed', err)
        toast.error(
          mapApiError(
            err,
            t('markdownEditor.uploadErrorFallback', { filename: file.name }),
          ).message,
        )
      }
    },
    [editorViewRef, maxAssetUploadSizeBytes, onDocChange, pageId, t],
  )

  // Handles paste requests.
  // This allows to paste images from clipboard directly into the editor.
  const handlePaste = useCallback(
    async (event: ClipboardEvent<HTMLDivElement>) => {
      const { clipboardData } = event
      if (!clipboardData) return

      const files: File[] = []

      if (clipboardData.files && clipboardData.files.length > 0) {
        files.push(...Array.from(clipboardData.files))
      } else if (clipboardData.items && clipboardData.items.length > 0) {
        Array.from(clipboardData.items).forEach((item) => {
          if (item.kind === 'file') {
            const file = item.getAsFile()
            if (file) {
              files.push(file)
            }
          }
        })
      }

      if (files.length === 0) {
        return
      }

      // We take over the paste event to handle image files
      event.preventDefault()
      event.stopPropagation()

      for (const file of files) {
        await uploadFileToEditor(file)
      }
    },
    [uploadFileToEditor],
  )

  // Dropping a file onto the editor is otherwise unhandled and falls through
  // to the browser's native contenteditable behavior — which inlines images
  // as bulky base64 data URIs and does nothing useful for a PDF or any other
  // file type. Intercept every file drop and upload through the same path as
  // paste, inserting at the drop point instead of wherever the text cursor
  // happens to be. Only the first file anchors to the drop point; the rest
  // chain off the cursor position paste itself just advanced to, the same
  // way a multi-file paste stacks its insertions.
  const handleDrop = useCallback(
    async (event: DragEvent<HTMLDivElement>) => {
      const files = Array.from(event.dataTransfer?.files ?? [])
      if (files.length === 0) {
        return
      }

      event.preventDefault()
      event.stopPropagation()

      const view = editorViewRef.current
      const dropPos =
        view?.posAtCoords({ x: event.clientX, y: event.clientY }) ??
        view?.state.selection.main.from

      for (let i = 0; i < files.length; i++) {
        await uploadFileToEditor(files[i], i === 0 ? dropPos : undefined)
      }
    },
    [editorViewRef, uploadFileToEditor],
  )

  return { handlePaste, handleDrop }
}
