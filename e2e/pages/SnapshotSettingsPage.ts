import { Page } from '@playwright/test';
import { toAppPath } from './appPath';

export default class SnapshotSettingsPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto(toAppPath('/settings/snapshots'));
    await this.page
      .getByRole('heading', { name: 'Full Backup', level: 1 })
      .waitFor({ state: 'visible' });
  }

  private snapshotRows() {
    return this.page.locator('.settings__preview', {
      has: this.page.getByRole('link', { name: 'Download' }),
    });
  }

  // Triggers a manual backup and waits for a new row to appear in the
  // snapshot list. The store awaits the create-backup request and reloads
  // the list before the button's own click handler returns, so no separate
  // "still running" state needs to be waited out here.
  async createBackup() {
    const before = await this.snapshotRows().count();
    await this.page.getByRole('button', { name: 'Create backup now' }).click();
    await this.snapshotRows().nth(before).waitFor({ state: 'visible' });
  }

  // Restores the most recently created snapshot (the list's first row) and
  // confirms the destructive AlertDialog. Restoring invalidates every
  // session including the caller's own, so the page ends up on /login (or
  // wherever the reload lands) once this resolves — callers must log back in
  // before doing anything else.
  async restoreLatest() {
    const row = this.snapshotRows().first();
    await row.getByRole('button', { name: 'Restore', exact: true }).click();

    const confirmDialog = this.page.getByRole('alertdialog');

    // The frontend polls the restore job to completion and then calls
    // window.location.reload() itself (SnapshotSettings.tsx) once the
    // restore (and its tail-end resync) finishes — wait for that navigation
    // rather than polling any status here. Registering the wait alongside
    // the click (not after it) matters: the click itself resolves almost
    // immediately, long before the reload actually fires.
    await Promise.all([
      this.page.waitForEvent('load', { timeout: 60_000 }),
      confirmDialog.getByRole('button', { name: 'Restore', exact: true }).click(),
    ]);
  }
}
