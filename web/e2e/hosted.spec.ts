import { expect, test, type APIRequestContext } from '@playwright/test';

const secret = process.env.E2E_TEST_LOGIN_SECRET || 'local';

async function rpc(request: APIRequestContext, token: string, method: string, body: unknown) {
  const res = await request.post(`/dailytasks.v1.${method}`, {
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    data: body,
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

const b64 = (s: string) => Buffer.from(s, 'utf8').toString('base64');
const unb64 = (s: string) => Buffer.from(s, 'base64').toString('utf8');

test('sign in, see tasks from another device, edit, and sync back', async ({ page, request }) => {
  const sub = `web-e2e-${Date.now()}`;

  // "Another device" stores two tasks for this account.
  const login = await request.get(`/auth/test?secret=${secret}&sub=${sub}&email=${sub}@example.test`);
  const { token } = await login.json();
  const today = new Date().toISOString().slice(0, 10);
  await rpc(request, token, 'SyncService/PutState', {
    data: b64(JSON.stringify({
      version: 1, last_reset: today, next_id: 3, theme_index: 0, last_modified: Date.now(),
      tasks: [
        { id: 1, title: 'Stretch', duration: 10, status: 'todo', order: 1 },
        { id: 2, title: 'Read 20 pages', duration: 25, status: 'todo', order: 2 },
      ],
    })),
    history: b64('{"version":1,"days":[],"events":[]}'),
  });

  // Unauthenticated visitors get the sign-in screen.
  await page.goto('/app/');
  await expect(page.getByTestId('sign-in-google')).toBeVisible();

  // Sign in through the same redirect flow Google would use.
  await page.goto(`/auth/test?secret=${secret}&sub=${sub}&email=${sub}@example.test&redirect_uri=${encodeURIComponent(new URL('/app/', page.url()).toString())}`);
  await expect(page).toHaveURL(/\/app\/$/); // token stripped from the address bar
  await expect(page.getByText('Read 20 pages')).toBeVisible();
  await page.screenshot({ path: 'e2e-out/web-board.png', fullPage: true });

  // Complete "Stretch" and check the server sees it.
  await page.getByRole('button', { name: 'Mark Stretch done' }).click();
  await expect.poll(async () => {
    const state = await rpc(request, token, 'SyncService/GetState', {});
    const data = JSON.parse(unb64(state.data));
    return data.tasks.find((t: { id: number }) => t.id === 1)?.status;
  }, { timeout: 10_000 }).toBe('done');

  // History recorded the change, so stats work across clients. It is pushed
  // right after the data document, so poll for it as well.
  await expect.poll(async () => {
    const state = await rpc(request, token, 'SyncService/GetState', {});
    const history = JSON.parse(unb64(state.history));
    return (history.events ?? []).some((e: { type: string; task_id: number }) => e.type === 'status_changed' && e.task_id === 1);
  }, { timeout: 10_000 }).toBe(true);
});
