import { fetchWithAuth } from './auth'

export type ImportPlan = {
  id: string
  tree_hash: string
  items: ImportPlanItem[]
  errors: string[]
}

export type ImportPlanItem = {
  source_path: string
  target_path: string
  title: string
  desired_slug: string
  kind: 'page' | 'section'
  exists: boolean
  existing_id: string | null
  action: 'create' | 'update' | 'skip'
  conflicts: string[] | null
  notes: string[] | null
}

export type ImportResult = {
  imported_count: number
  updated_count: number
  skipped_count: number
  items: {
    source_path: string
    target_path: string
    action: 'created' | 'updated' | 'skipped' | 'conflicted'
    error?: string
  }[]
  tree_hash: string
  tree_hash_before: string
}

export async function createImportPlanFromZip(file: File): Promise<ImportPlan> {
  const formData = new FormData()
  formData.append('file', file)

  return (await fetchWithAuth('/api/import/plan', {
    method: 'POST',
    body: formData,
    headers: {}, // Let browser set Content-Type for FormData
  })) as Promise<ImportPlan>
}

export async function getImportPlan(): Promise<ImportPlan> {
  return (await fetchWithAuth('/api/import/plan', {
    method: 'GET',
  })) as Promise<ImportPlan>
}

export async function executeImportPlan(): Promise<ImportResult> {
  return (await fetchWithAuth('/api/import/execute', {
    method: 'POST',
  })) as Promise<ImportResult>
}
