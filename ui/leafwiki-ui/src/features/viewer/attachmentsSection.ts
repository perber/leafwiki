import type { PageAttachment } from '@/lib/api/assets'
import { withBasePath } from '@/lib/routePath'
import type { PagePanelSection } from '../preview/pagePanelSection'

export function buildAttachmentsSection(
  attachments: PageAttachment[],
): PagePanelSection {
  return {
    id: 'attachments',
    titleKey: 'toc.downloads',
    items: attachments.map((file) => ({
      kind: 'download',
      key: file.url,
      label: file.name,
      href: withBasePath(file.url),
      fileName: file.name,
      testId: `download-${file.name}`,
    })),
  }
}
