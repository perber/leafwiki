import { Page } from '@playwright/test';

export type BrowserResponse<T> = { ok: boolean; status: number; body: T };

// browserFetch runs fetch inside the page, so the request carries the browser
// session's cookies (and base path) exactly like the app's own requests. Every
// request sends the CSRF header from the session's CSRF cookie; a JSON body
// also sets Content-Type. A non-JSON or empty response body yields null.
export async function browserFetch<T = unknown>(
  page: Page,
  path: string,
  init: { method?: string; body?: unknown } = {},
): Promise<BrowserResponse<T>> {
  return page.evaluate(
    async ({ path, method, body, hasBody }) => {
      const match =
        document.cookie.match(/(?:^|;\s*)__Host-leafwiki_csrf=([^;]+)/) ??
        document.cookie.match(/(?:^|;\s*)leafwiki_csrf=([^;]+)/);
      if (!match) throw new Error('Missing CSRF token cookie');
      let csrfToken: string;
      try {
        csrfToken = decodeURIComponent(match[1]);
      } catch {
        csrfToken = match[1];
      }

      const headers: Record<string, string> = { 'X-CSRF-Token': csrfToken };
      if (hasBody) headers['Content-Type'] = 'application/json';
      const res = await fetch(path, {
        method,
        credentials: 'include',
        headers,
        body: hasBody ? JSON.stringify(body) : undefined,
      });
      const text = await res.text();
      let parsed: unknown;
      try {
        parsed = text ? JSON.parse(text) : null;
      } catch {
        parsed = null;
      }
      return { ok: res.ok, status: res.status, body: parsed as T };
    },
    { path, method: init.method ?? 'GET', body: init.body, hasBody: init.body !== undefined },
  );
}

// expectOk throws `${label} failed: <status>` for a non-2xx response.
export function expectOk<T>(res: BrowserResponse<T>, label: string): T {
  if (!res.ok) throw new Error(`${label} failed: ${res.status}`);
  return res.body;
}

export function pageByPathUrl(path: string): string {
  return `/api/pages/by-path?path=${encodeURIComponent(path)}`;
}

export async function createPage(
  page: Page,
  input: { title: string; slug: string; content?: string },
) {
  const { title, slug, content } = input;
  const created = expectOk(
    await browserFetch<{ id: string; version: string }>(page, '/api/pages', {
      method: 'POST',
      body: { parentId: null, title, slug, kind: 'page' },
    }),
    'create',
  );
  if (content !== undefined) {
    expectOk(
      await browserFetch(page, `/api/pages/${created.id}`, {
        method: 'PUT',
        body: { version: created.version, title, slug, content, tags: [], properties: {} },
      }),
      'update',
    );
  }
}
