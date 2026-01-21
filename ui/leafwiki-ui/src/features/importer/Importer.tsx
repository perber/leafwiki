import { Button } from '@/components/ui/button'
import { useImportStore } from '@/stores/import'
import { FileUp, Loader2, PlayIcon, UploadIcon } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useSetTitle } from '../viewer/setTitle'
import { useToolbarActions } from './useToolbarActions'

export default function Importer() {
  // reset toolbar actions on mount
  useToolbarActions()
  useSetTitle({ title: 'Import' })
  const zipRef = useRef<HTMLInputElement>(null)
  const [zipFileName, setZipFileName] = useState('')

  const createImportPlan = useImportStore((store) => store.createImportPlan)
  const loadImportPlan = useImportStore((store) => store.loadImportPlan)
  const executeImportPlan = useImportStore((store) => store.executeImportPlan)
  const creatingImportPlan = useImportStore((store) => store.creatingImportPlan)
  const executingImportPlan = useImportStore(
    (store) => store.executingImportPlan,
  )

  const importPlan = useImportStore((store) => store.importPlan)

  useEffect(() => {
    loadImportPlan()
  }, [loadImportPlan])

  const createImportPlanFromZip = useCallback(() => {
    const zipFile = zipRef.current?.files?.[0]
    if (!zipFile) {
      return
    }
    createImportPlan(zipFile)
  }, [createImportPlan])

  return (
    <>
      <div className="settings importer">
        <h1 className="settings__title">Import</h1>
        <div className="settings__section">
          <h2 className="settings__section-title">Import Zip File with Markdown Files</h2>
          <p className="settings__section-description">
            Import Markdown Files from a Zip. <br /><br />
            <b>Note:</b> Links between pages will not be updated automatically and the import is only importing markdown files. <br />
            You may need to adjust links and add images manually after the import. <br /><br />

            If you require a more advanced import solution, please create an issue on our <a href="https://github.com/perber/leafwiki/issues" target="_blank" rel="noopener noreferrer">GitHub repository</a>.
          </p>
          <div className="settings__preview">
            <span className="settings__preview-label">Current Zip:</span>
            {zipFileName ? (
              <>
                <span className="settings__preview-filename">
                  {zipFileName}
                </span>
              </>
            ) : (
              <span className="settings__preview-placeholder">
                No zip file selected
              </span>
            )}
          </div>
          <div className="settings__field">
            <input
              type="file"
              ref={zipRef}
              accept=".zip"
              className="hidden"
              onChange={(e) => {
                const file = e.target.files?.[0]
                setZipFileName(file?.name ?? '')
              }}
            />
            <Button
              variant="outline"
              onClick={() => {
                zipRef.current?.click()
              }}
            >
              <FileUp className="mr-2 h-4 w-4" />
              Select Zip File
            </Button>
          </div>
          <Button
            variant="default"
            className="settings__save-button"
            onClick={createImportPlanFromZip}
            disabled={!zipFileName || creatingImportPlan}
          >
            {creatingImportPlan ? (
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            ) : (
              <UploadIcon className="mr-2 h-4 w-4" />
            )}
            Import from Zip
          </Button>
        </div>
        {importPlan && (
          <div className="settings__section">
            <h2 className="settings__section-title">Import Plan</h2>

            <div className="settings__table-card">
              <div className="settings__table-scroll">
                <table className="settings__table">
                  <tbody>
                    <tr>
                      <th className="settings__table-header-cell">ID:</th>
                      <td>{importPlan.id}</td>
                    </tr>
                    <tr>
                      <th className="settings__table-header-cell">
                        Tree Hash:
                      </th>
                      <td>{importPlan.tree_hash}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        )}
        ️
        {importPlan && importPlan.items.length > 0 && (
          <>
            <div className="settings__section">
              <h2 className="settings__section-title">Items ({importPlan.items.length})</h2>
              <div className="settings__field">
                <div className="settings__table-card">
                  <div className="settings__table-scroll">
                    <table className="settings__table">
                      <thead className="settings__table-head">
                        <tr>
                          <th className="settings__table-header-cell">
                            Source Path
                          </th>
                          <th className="settings__table-header-cell">
                            Target Path
                          </th>
                          <th className="settings__table-header-cell">Title</th>
                          <th className="settings__table-header-cell">Kind</th>
                          <th className="settings__table-header-cell">
                            Action
                          </th>
                          <th className="settings__table-header-cell">Notes</th>
                        </tr>
                      </thead>
                      <tbody>
                        {importPlan.items.map((item, index) => (
                          <tr key={index}>
                            <td className="settings__table-cell">
                              {item.source_path}
                            </td>
                            <td className="settings__table-cell">
                              {item.target_path}
                            </td>
                            <td className="settings__table-cell">
                              {item.title}
                            </td>
                            <td className="settings__table-cell">
                              <span className="settings__pill settings__pill-success">{item.kind}</span>
                            </td>
                            <td className="settings__table-cell">
                              <span className="settings__pill settings__pill-success">{item.action}</span>
                            </td>
                            <td className="settings__table-cell">
                              {item.notes ? item.notes.join(', ') : ''}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              </div>
              <Button
                variant="default"
                onClick={() => {
                  executeImportPlan()
                }}
                disabled={executingImportPlan || creatingImportPlan}
              >
                {executingImportPlan ? (
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <PlayIcon className="mr-2 h-4 w-4" />
                )}
                Execute Import Plan
              </Button>
            </div>
          </>
        )}
      </div>
    </>
  )
}
