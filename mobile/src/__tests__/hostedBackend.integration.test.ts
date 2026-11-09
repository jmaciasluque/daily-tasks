// Runs the real Connect client against a real server. Skipped unless
// DAILY_TASKS_IT_URL and DAILY_TASKS_IT_TOKEN are set, e.g.:
//
//   DEV_MEMORY_STORE=1 TEST_LOGIN_SECRET=local ... go run ./cmd/server   (in server/)
//   DAILY_TASKS_IT_URL=http://localhost:8080 \
//   DAILY_TASKS_IT_TOKEN=$(curl -s 'http://localhost:8080/auth/test?secret=local&sub=it' | jq -r .token) \
//   npx jest hostedBackend.integration
jest.mock('../config/env', () => ({ hostedApiUrl: 'unused', appVersion: 'it' }));

import { HostedBackend } from '../services/backend_hosted';
import { fetchRemoteData, syncWithRemoteState } from '../services/sync';
import { EtagMismatchError, KeyData } from '../services/backend';
import { emptyHistory } from '../services/history';

const url = process.env.DAILY_TASKS_IT_URL;
const token = process.env.DAILY_TASKS_IT_TOKEN;
const run = url && token ? describe : describe.skip;

run('HostedBackend against a live server', () => {
  it('syncs data and history end to end', async () => {
    const backend = new HostedBackend({ apiUrl: url!, token });
    const local = {
      version: 1,
      last_reset: '2026-11-09',
      next_id: 2,
      theme_index: 0,
      last_modified: Date.now(),
      tasks: [{ id: 1, title: 'Integración ✓', duration: 15, status: 'todo' as const, order: 1 }],
    };
    const result = await syncWithRemoteState(backend, local, emptyHistory());
    expect(result.action).not.toBe('error');

    const { data, etag } = await fetchRemoteData(backend);
    expect(data?.tasks[0].title).toBe('Integración ✓');
    expect(etag).toMatch(/^"sync-\d+"$/);

    await expect(backend.push(KeyData, '{}', '"sync-1"')).rejects.toBeInstanceOf(EtagMismatchError);
    await expect(backend.getAccount()).resolves.toMatchObject({ provider: 'test' });
  });
});
