import { test } from '@playwright/test';
import LoginPage from '../pages/LoginPage';
import SnapshotSettingsPage from '../pages/SnapshotSettingsPage';
import UserManagementPage from '../pages/UserManagementPage';
import ViewPage from '../pages/ViewPage';

const user = process.env.E2E_ADMIN_USER || 'admin';
const password = process.env.E2E_ADMIN_PASSWORD || 'admin';

// This test triggers a real full-backup live restore against the shared e2e
// instance every other spec file also runs against. A restore invalidates
// every session (including this test's own — see restore.json's
// restoreConfirmDescription) and rolls back all users/pages/settings to
// exactly the snapshot's content. To keep the blast radius on the rest of
// the suite at zero, the snapshot is taken immediately before creating one
// throwaway user, and the restore rolls back to exactly that snapshot — so
// nothing that existed before this test is ever at risk, and the throwaway
// user this test adds doesn't outlive it either. If more tests are ever
// added to this file, keep them serial: two restores racing each other
// against the same instance is not something the backend guards against
// gracefully (ErrRestoreAlreadyRunning aside).
test('user management reflects a live restore without a server restart', async ({ page }) => {
  const throwawayUsername = `e2e-restore-check-${Date.now()}`;

  const loginPage = new LoginPage(page);
  const viewPage = new ViewPage(page);
  await loginPage.goto();
  await loginPage.login(user, password);
  await viewPage.expectUserLoggedIn();

  const snapshots = new SnapshotSettingsPage(page);
  await snapshots.goto();
  await snapshots.createBackup();

  const users = new UserManagementPage(page);
  await users.goto();
  await users.createUser(throwawayUsername, `${throwawayUsername}@example.com`, 'password123');
  await users.expectUserPresent(throwawayUsername);

  // Regression check for "User-Management Routes Go Stale After Live
  // Restore": before the fix, the admin user-management routes kept
  // operating against the pre-restore users.db forever after a live
  // restore, because they were wired from a *UserService pointer captured
  // once at server boot instead of resolved through AuthService on every
  // call. Restoring the snapshot taken above (before the throwaway user
  // existed) and then confirming that user is gone — through the real
  // HTTP/DB stack, with no server restart in between — is exactly what that
  // bug broke.
  await snapshots.goto();
  await snapshots.restoreLatest();

  // The restore just revoked every session, including this one.
  await loginPage.goto();
  await loginPage.login(user, password);
  await viewPage.expectUserLoggedIn();

  await users.goto();
  await users.expectUserAbsent(throwawayUsername);
});
