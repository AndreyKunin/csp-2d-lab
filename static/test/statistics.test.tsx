// FE-05: the statistics panels' Duration advances with every status update.
// FE-06: the window chart plots the window's elite values.

import './fakedom';
import {test} from 'node:test';
import * as assert from 'node:assert/strict';
import * as React from 'react';
import {act} from 'react-dom/test-utils';
import {createRoot} from 'react-dom/client';
import {fakeDocument} from './fakedom';
import {LabStatisticsPanel} from '../src/components/lab/LabStatisticsPanel';
import {MonitorStatisticsPanel} from '../src/components/lab/MonitorStatisticsPanel';
import {ChartsMediator} from '../src/services/charts';
import {DashboardState} from '../src/services/status';
import {SimulationStatistics} from '../src/utils/simulation';

// A dashboard state whose status updates the test fires, as a status poll does.
function fakeDashboard() {
    const subscribers: (() => void)[] = [];
    let epoch = 1;
    const dashboard: any = {
        getStatus: () => 'running',
        getStatistics: () => ({state: {epochState: {epochNumber: epoch}, windowState: {}, cycles: 0}, parameters: {maxEpochsCount: 100}}),
        getRuntimeParameters: () => undefined,
        onStatusChange: (cb: () => void) => { subscribers.push(cb); return () => {}; },
    };
    const poll = async (nextEpoch: number) => {
        epoch = nextEpoch;
        await act(async () => { subscribers.forEach(cb => cb()); });
    };
    return {dashboard, poll};
}

// The text of every element of the container, depth first.
function texts(node: any): string[] {
    const out: string[] = [];
    const walk = (n: any) => {
        if (n.nodeType === 1 && n.childNodes.length === 1 && n.childNodes[0].nodeType === 3) {
            out.push(n.textContent);
        }
        n.childNodes.forEach(walk);
    };
    walk(node);
    return out;
}

for (const [name, Panel] of [['LabStatisticsPanel', LabStatisticsPanel], ['MonitorStatisticsPanel', MonitorStatisticsPanel]] as const) {
    test(`${name}: Duration advances with every status update`, async () => {
        const realNow = Date.now;
        let now = 1_000_000;
        Date.now = () => now;
        try {
            const {dashboard, poll} = fakeDashboard();
            const container = fakeDocument.createElement('div');
            const root = createRoot(container);
            await act(async () => {
                root.render(React.createElement(Panel as any, {dashboardState: dashboard, startTime: new Date(now)}));
            });
            assert.ok(texts(container).includes('00:00:00'), texts(container).join('|'));

            now += 5000;
            await poll(7);
            assert.ok(texts(container).includes('7'), 'the generation number updated');
            assert.ok(texts(container).includes('00:00:05'), `after 5 s: ${texts(container).join('|')}`);

            now += 3_600_000;
            await poll(90);
            assert.ok(texts(container).includes('01:00:05'), `after 1 h 5 s: ${texts(container).join('|')}`);
            await act(async () => root.unmount());
        } finally {
            Date.now = realNow;
        }
    });
}

test('the window chart plots the window values, the epoch chart the epoch values', () => {
    const mediator = new ChartsMediator(new DashboardState('abc', 'running', true, () => {}));
    const statistics: SimulationStatistics = {
        state: {
            epochState: {bestFitness: 1, averageFitness: 2, fitnessDeviation: 3, averageEliteFitness: 4, eliteFitnessDeviation: 5, epochNumber: 9, failures: 0},
            windowState: {averageFitness: 20, averageFitnessDeviation: 30, averageEliteFitness: 40, eliteFitnessDeviation: 50, saturationRate: 0.1},
            cycles: 0, championshipSince: 0,
        },
    } as SimulationStatistics;
    const at = new Date(0);
    const window = (mediator as any).getWindow(at, statistics);
    assert.deepEqual([window.bestSeriesY, window.avgSeriesY, window.devSeriesY, window.avgESeriesY, window.devESeriesY],
        [null, 20, 30, 40, 50]);
    const epoch = (mediator as any).getEpoch(at, statistics);
    assert.deepEqual([epoch.bestSeriesY, epoch.avgSeriesY, epoch.devSeriesY, epoch.avgESeriesY, epoch.devESeriesY],
        [1, 2, 3, 4, 5]);
    mediator.dispose();
});
