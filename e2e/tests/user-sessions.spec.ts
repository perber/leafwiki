import { APIRequestContext, expect, test } from '@playwright/test';
import { adminPassword, adminUser, csrfHeader, loggedIn } from '../helpers/apiContext';

// A revoked refresh token is rejected with 422 (invalid refresh token).
const REVOKED = 422;

async function refresh(ctx: APIRequestContext) {
  return (await ctx.post('/api/auth/refresh-token', { headers: await csrfHeader(ctx) })).status();
}

test.describe('user update and session revocation', () => {
  const password = 'initial-password-123';
  let admin: APIRequestContext;
  let editorId: string;
  let username: string;

  test.beforeEach(async () => {
    username = `editor${Date.now()}`;
    admin = await loggedIn(adminUser, adminPassword);
    const created = await admin.post('/api/users', {
      headers: await csrfHeader(admin),
      data: { username, email: `${username}@example.com`, password, role: 'editor' },
    });
    expect(created.status()).toBe(201);
    editorId = (await created.json()).id;
  });

  test.afterEach(async () => {
    await admin.delete(`/api/users/${editorId}`, { headers: await csrfHeader(admin) });
    await admin.dispose();
  });

  test('a non-admin cannot update their own user record', async () => {
    const editor = await loggedIn(username, password);
    const resp = await editor.put(`/api/users/${editorId}`, {
      headers: await csrfHeader(editor),
      data: { username, email: 'changed@example.com', role: 'admin' },
    });
    expect(resp.status()).toBe(403);
    await editor.dispose();
  });

  test('changing the own password ends other sessions but keeps the current one', async () => {
    const current = await loggedIn(username, password);
    const other = await loggedIn(username, password);

    const resp = await current.put('/api/users/me/password', {
      headers: await csrfHeader(current),
      data: { oldPassword: password, newPassword: 'changed-password-456' },
    });
    expect(resp.status()).toBe(204);

    expect(await refresh(other)).toBe(REVOKED);
    expect(await refresh(current)).toBe(200);

    await current.dispose();
    await other.dispose();
  });

  test('an admin setting a password ends all sessions of that user', async () => {
    const editor = await loggedIn(username, password);

    const resp = await admin.put(`/api/users/${editorId}`, {
      headers: await csrfHeader(admin),
      data: {
        username,
        email: `${username}@example.com`,
        role: 'editor',
        password: 'admin-set-password-789',
      },
    });
    expect(resp.status()).toBe(200);

    expect(await refresh(editor)).toBe(REVOKED);
    await editor.dispose();
  });
});
