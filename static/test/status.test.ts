// FE-04: when does finishMonitoring notify the status subscribers? PlayerPanel's
// subscriber runs finishJob (fetch the result, delete the job) when the status
// becomes "completed"; onStop also runs it explicitly.

import {test} from 'node:test';
import * as assert from 'node:assert/strict';
import {DashboardState} from '../src/services/status';

// DashboardState polls with window.setInterval; Node has no window.
(globalThis as any).window = {setInterval: () => 1, clearInterval: () => {}};

test('before the first status poll, finishMonitoring does not notify: only the explicit finishJob of onStop runs', async () => {
    const state = new DashboardState('abc', 'running', true, () => {});
    let notified = 0;
    state.onStatusChange(() => notified++);
    assert.equal(state.getStatus(), 'running'); // the initial status: Stop is enabled
    await state.finishMonitoring();
    assert.equal(notified, 0);
    assert.equal(state.getStatus(), 'running');
});

test('after a status poll, finishMonitoring flips the status to completed and notifies', async () => {
    const state = new DashboardState('abc', 'running', true, () => {});
    (state as any).jobStatus = {status: 'paused'}; // what a poll stores
    let notified = 0;
    state.onStatusChange(() => notified++);
    await state.finishMonitoring();
    assert.equal(notified, 1);
    assert.equal(state.getStatus(), 'completed');
});

// finishJob is triggered twice on Stop (the status subscriber and onStop), and
// both triggers are needed: before the first status poll only onStop's runs
// (the test above). finishOnce makes them one run.
import {finishOnce} from '../src/services/status';

test('finishOnce: two concurrent calls run the finish once and share its outcome', async () => {
    let runs = 0;
    let release!: () => void;
    const finish = finishOnce(() => {
        runs++;
        return new Promise<void>(resolve => { release = resolve; });
    });
    const first = finish();   // the status subscriber
    const second = finish();  // onStop
    release();
    await Promise.all([first, second]);
    assert.equal(runs, 1);
});

test('finishOnce: a call after the finish succeeded does not run it again', async () => {
    let runs = 0;
    const finish = finishOnce(async () => { runs++; });
    await finish();
    await finish();
    assert.equal(runs, 1);
});

test('finishOnce: concurrent callers share a failure, and a later call may retry', async () => {
    let runs = 0;
    const finish = finishOnce(async () => {
        runs++;
        if (runs === 1) {
            throw new Error('Server error! Status: 500.');
        }
    });
    const results = await Promise.allSettled([finish(), finish()]);
    assert.deepEqual(results.map(r => r.status), ['rejected', 'rejected']);
    assert.equal(runs, 1);
    await finish(); // Stop again: the job is still there
    assert.equal(runs, 2);
});
