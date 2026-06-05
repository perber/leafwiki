import { Page } from '@playwright/test';

export function getCsrfScript(): string {
  return `
    const hostMatch =
      document.cookie.match(/(?:^|;\\s*)__Host-leafwiki_csrf=([^;]+)/) ??
      document.cookie.match(/(?:^|;\\s*)leafwiki_csrf=([^;]+)/);
    if (!hostMatch) throw new Error('Missing CSRF token cookie');
    try { return decodeURIComponent(hostMatch[1]); } catch { return hostMatch[1]; }
  `;
}

export async function createPage(
  page: Page,
  input: { title: string; slug: string; content?: string },
) {
  await page.evaluate(
    async ({ title, slug, content, csrfScript }) => {
      const csrfToken = new Function(csrfScript)() as string;

      const createRes = await fetch('/api/pages', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
        body: JSON.stringify({ parentId: null, title, slug, kind: 'page' }),
      });
      if (!createRes.ok) throw new Error(`create failed: ${createRes.status}`);

      if (content !== undefined) {
        const created = (await createRes.json()) as { id: string; version: string };
        const updateRes = await fetch(`/api/pages/${created.id}`, {
          method: 'PUT',
          credentials: 'include',
          headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
          body: JSON.stringify({ version: created.version, title, slug, content, tags: [], properties: {} }),
        });
        if (!updateRes.ok) throw new Error(`update failed: ${updateRes.status}`);
      }
    },
    { ...input, csrfScript: getCsrfScript() },
  );
}
