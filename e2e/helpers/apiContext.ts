import { APIRequestContext, expect, request } from '@playwright/test';

const baseURL = process.env.E2E_BASE_URL || 'http://localhost:8080';
export const adminUser = process.env.E2E_ADMIN_USER || 'admin';
export const adminPassword = process.env.E2E_ADMIN_PASSWORD || 'admin';

export async function csrfHeader(ctx: APIRequestContext) {
  const { cookies } = await ctx.storageState();
  const csrf = cookies.find((c) => c.name.endsWith('leafwiki_csrf'));
  return { 'X-CSRF-Token': csrf ? decodeURIComponent(csrf.value) : '' };
}

// loggedIn returns a standalone API context with its own session cookies.
export async function loggedIn(identifier = adminUser, password = adminPassword) {
  const ctx = await request.newContext({ baseURL });
  const resp = await ctx.post('/api/auth/login', { data: { identifier, password } });
  expect(resp.status(), `login as ${identifier}`).toBe(200);
  return ctx;
}

// createPageViaApi creates a root-level page and returns its id.
export async function createPageViaApi(
  ctx: APIRequestContext,
  input: { title: string; slug: string; content: string },
) {
  const headers = await csrfHeader(ctx);
  const created = await ctx.post('/api/pages', {
    headers,
    data: { parentId: null, title: input.title, slug: input.slug, kind: 'page' },
  });
  expect(created.status(), `create page ${input.slug}`).toBe(201);
  const { id, version } = await created.json();
  const updated = await ctx.put(`/api/pages/${id}`, {
    headers,
    data: {
      version,
      title: input.title,
      slug: input.slug,
      content: input.content,
      tags: [],
      properties: {},
    },
  });
  expect(updated.ok(), `update page ${input.slug}`).toBeTruthy();
  return id as string;
}
