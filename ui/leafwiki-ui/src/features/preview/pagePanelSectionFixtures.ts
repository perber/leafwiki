import type { PageAttachment } from '@/lib/api/assets'
import type { PagePanelSection } from './pagePanelSection'

// Test fixture for the panel tests: a download section shaped like the one
// features/viewer/attachmentsSection.ts builds, without importing the viewer
// from preview (the dependency points viewer → preview).
export function downloadsSection(files: PageAttachment[]): PagePanelSection {
  return {
    id: 'attachments',
    titleKey: 'toc.downloads',
    items: files.map((file) => ({
      kind: 'download',
      key: file.url,
      label: file.name,
      href: file.url,
      fileName: file.name,
      testId: `download-${file.name}`,
    })),
  }
}
