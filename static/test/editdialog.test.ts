// FE-02: the Lab Edit dialog checks the values against their rules and against
// the running job (its population size, its window size), says why a value is
// rejected, and shows a failed configure instead of losing it.

import {test} from 'node:test';
import * as assert from 'node:assert/strict';
import {ServerParameterDefinitions, setParameterRules} from '../src/utils/parameterDefinitions';
import {getRuntimeParameters, PropertyItem, submitRuntimeParameters, validateRuntimeParameters} from '../src/utils/parameters';
import {SimulationParameters, SimulationRuntimeParameters} from '../src/utils/simulation';

const SERVER: ServerParameterDefinitions = {
    'populationSize.min': {default: 100, rule: 'int[3-)'}, 'populationSize.max': {default: 60000, rule: 'int[3-)'},
    eliteCount: {default: 3, rule: 'int[1-)'}, mutationRate: {default: 0.07, rule: 'float[0-1]'},
    crossoverRate: {default: 0.75, rule: 'float(0-1]'}, maxEpochsCount: {default: 20000, rule: 'int[1-)'},
    windowSize: {default: 50, rule: 'int[1-250]'}, cycleCount: {default: 10, rule: 'int[1-)'},
    tournamentSize: {default: 5, rule: 'int[2-)'}, repairWeight: {default: 3, rule: 'int[1-)'},
    flipWeight: {default: 1, rule: 'int[1-)'}, donorFraction: {default: 0.5, rule: 'float(0-1]'},
};

// The running job: population 60, window 50.
const RUNNING: SimulationRuntimeParameters = {
    populationSize: 60, eliteCount: 3, mutationRate: 0.07, crossoverRate: 0.75, maxEpochsCount: 20000,
    windowSize: 50, cycleCount: 10, tournamentSize: 5, repairWeight: 3, flipWeight: 1, donorFraction: 0.5,
};

// dialog returns the Edit dialog's items with some values changed.
function dialog(changes: { [key: string]: string }): PropertyItem[] {
    setParameterRules(SERVER);
    return getRuntimeParameters(RUNNING).map(item => key(item, changes));
}

function key(item: PropertyItem, changes: { [key: string]: string }): PropertyItem {
    return item.key in changes ? {...item, value: changes[item.key]} : item;
}

test('the running values are valid', () => {
    assert.deepEqual(validateRuntimeParameters(dialog({}), RUNNING), []);
});

test('an invalid value says why', () => {
    assert.deepEqual(validateRuntimeParameters(dialog({mutationRate: '1.5', tournamentSize: ''}), RUNNING), [
        'Mutation rate must be a number from 0 to 1.',
        'Tournament size must be a whole number, at least 2.',
    ]);
});

test('the elite count is below the running population size', () => {
    assert.deepEqual(validateRuntimeParameters(dialog({eliteCount: '59'}), RUNNING), []);
    assert.deepEqual(validateRuntimeParameters(dialog({eliteCount: '60'}), RUNNING),
        ['Elite count must be less than the population size (60).']);
    assert.deepEqual(validateRuntimeParameters(dialog({eliteCount: '1000'}), RUNNING),
        ['Elite count must be less than the population size (60).']);
});

test('the number of generations is at least the running window size', () => {
    assert.deepEqual(validateRuntimeParameters(dialog({maxEpochsCount: '50'}), RUNNING), []);
    assert.deepEqual(validateRuntimeParameters(dialog({maxEpochsCount: '49'}), RUNNING),
        ['Number of generations must be at least the window size (50).']);
});

test('the relations are checked only when every value is valid', () => {
    assert.deepEqual(validateRuntimeParameters(dialog({eliteCount: '0', maxEpochsCount: '49'}), RUNNING),
        ['Elite count must be a whole number, at least 1.']);
});

test('valid values are submitted; nothing is shown', async () => {
    let sent: SimulationParameters | undefined;
    const messages = await submitRuntimeParameters(dialog({mutationRate: '0.2'}), RUNNING, async p => { sent = p; });
    assert.deepEqual(messages, []);
    assert.equal(sent!.mutationRate, 0.2);
});

test('invalid values are not submitted', async () => {
    let called = false;
    const messages = await submitRuntimeParameters(dialog({eliteCount: '60'}), RUNNING, async () => { called = true; });
    assert.equal(called, false);
    assert.deepEqual(messages, ['Elite count must be less than the population size (60).']);
});

test('a failed configure is shown, not lost', async () => {
    const messages = await submitRuntimeParameters(dialog({}), RUNNING, async () => {
        throw new Error('Server error! Status: 409. simulation abc is completed');
    });
    assert.deepEqual(messages, ['Server error! Status: 409. simulation abc is completed']);
});
