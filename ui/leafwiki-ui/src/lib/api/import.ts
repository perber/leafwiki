import { fetchWithAuth } from './auth'

/**
 * 
 *{
    "id": "q9cEcXSDR",
    "tree_hash": "028bc386f9f8f94b66d240430a4b4fe90e26879d67041444ed854b4f289b100f",
    "items": [
        {
            "source_path": "features/index.md",
            "target_path": "features",
            "title": "Features",
            "desired_slug": "features",
            "kind": "section",
            "exists": false,
            "existing_id": null,
            "action": "create",
            "conflicts": null,
            "notes": null
        },
        {
            "source_path": "features/mermaind.md",
            "target_path": "features/mermaind",
            "title": "Mermaid",
            "desired_slug": "mermaind",
            "kind": "page",
            "exists": false,
            "existing_id": null,
            "action": "create",
            "conflicts": null,
            "notes": null
        },
        {
            "source_path": "home.md",
            "target_path": "home",
            "title": "Home",
            "desired_slug": "home",
            "kind": "page",
            "exists": false,
            "existing_id": null,
            "action": "create",
            "conflicts": null,
            "notes": null
        }
    ],
    "errors": []
}
 */

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
  action: 'create' | 'update' | 'skip' | 'conflict'
  conflicts: string[] | null
  notes: string[] | null
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

export async function executeImportPlan(): Promise<void> {
  await fetchWithAuth('/api/import/execute', {
    method: 'POST',
  })
}
