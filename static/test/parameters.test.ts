// FE-07: the parameter rules the server sends, their checks, and the parameter
// lists of the start form and the Edit dialog (src/utils/parameterDefinitions.ts,
// parameters.ts, simulation.ts).

import {test} from 'node:test';
import * as assert from 'node:assert/strict';
import {
    checkRuleValue, defaultParameterValues, describeRule, findParameter, overriddenParameters, parseRule,
    PARAMETER_DEFINITIONS, ServerParameterDefinitions, setParameterRules, validateParameter, valueOf
} from '../src/utils/parameterDefinitions';
import {getInitialParameters, getRuntimeParameters, getSimulationParameters} from '../src/utils/parameters';
import {Simulation, SimulationRuntimeParameters} from '../src/utils/simulation';

// The production definitions (application.yaml): the server's response.
const SERVER: ServerParameterDefinitions = {
    'populationSize.min': {default: 100, rule: 'int[3-)'}, 'populationSize.max': {default: 60000, rule: 'int[3-)'},
    eliteCount: {default: 3, rule: 'int[1-)'}, mutationRate: {default: 0.07, rule: 'float[0-1]'},
    crossoverRate: {default: 0.75, rule: 'float(0-1]'}, maxEpochsCount: {default: 20000, rule: 'int[1-)'},
    windowSize: {default: 50, rule: 'int[1-250]'}, cycleCount: {default: 10, rule: 'int[1-)'},
    tournamentSize: {default: 5, rule: 'int[2-)'}, repairWeight: {default: 3, rule: 'int[1-)'},
    flipWeight: {default: 1, rule: 'int[1-)'}, donorFraction: {default: 0.5, rule: 'float(0-1]'},
};

test('parseRule reads the DSL; malformed rules are rejected', () => {
    assert.deepEqual(parseRule('int[1-250]'), {type: 'int', min: 1, minInclusive: true, max: 250, maxInclusive: true});
    assert.deepEqual(parseRule('float(0-1]'), {type: 'float', min: 0, minInclusive: false, max: 1, maxInclusive: true});
    assert.deepEqual(parseRule('int[2-)'), {type: 'int', min: 2, minInclusive: true, max: Infinity, maxInclusive: false});
    for (const bad of ['', 'int', 'integer[1-)', 'int[1-', 'int[5-3]', 'int[a-3]']) {
        assert.throws(() => parseRule(bad), /Invalid parameter rule/, bad);
    }
});

test('describeRule and checkRuleValue word and check the bounds', () => {
    assert.equal(describeRule(parseRule('int[1-250]')), 'a whole number from 1 to 250');
    assert.equal(describeRule(parseRule('float(0-1]')), 'a number, greater than 0 and at most 1');
    assert.equal(describeRule(parseRule('int[2-)')), 'a whole number, at least 2');
    const window = parseRule('int[1-250]');
    for (const ok of ['1', '250', ' 7 ']) {
        assert.equal(checkRuleValue(window, ok), null, ok);
    }
    for (const bad of ['0', '251', '1.5', 'x', '']) {
        assert.equal(checkRuleValue(window, bad), 'must be a whole number from 1 to 250', bad);
    }
    const rate = parseRule('float(0-1]');
    assert.equal(checkRuleValue(rate, '0'), 'must be a number, greater than 0 and at most 1');
    assert.equal(checkRuleValue(rate, '.5'), null);
    assert.equal(checkRuleValue(rate, '1'), null);
});

test('without the server rules a parameter cannot be validated; with them it can', () => {
    setParameterRules({});
    assert.equal(validateParameter('eliteCount', '1'), 'No validation rule for eliteCount from the server.');
    assert.equal(findParameter('eliteCount'), undefined);
    setParameterRules(SERVER);
    assert.equal(validateParameter('eliteCount', '1'), null);
    assert.equal(validateParameter('eliteCount', '0'), 'Elite count must be a whole number, at least 1.');
    assert.equal(validateParameter('windowSize', '251'), 'Window size must be a whole number from 1 to 250.');
    assert.equal(validateParameter('nope', '1'), 'Unknown parameter nope.');
    assert.equal(findParameter('windowSize')!.definition.displayName, 'Window size');
});

test('every definition has a server rule', () => {
    setParameterRules(SERVER);
    for (const d of PARAMETER_DEFINITIONS) {
        assert.ok(findParameter(d.key), `${d.key} has no rule`);
    }
    assert.equal(PARAMETER_DEFINITIONS.length, Object.keys(SERVER).length);
});

test('defaultParameterValues nests dotted keys', () => {
    const v = defaultParameterValues(SERVER) as any;
    assert.deepEqual(v.populationSize, {min: 100, max: 60000});
    assert.equal(v.eliteCount, 3);
    assert.equal(valueOf(v, 'populationSize.max'), 60000);
    assert.equal(valueOf(v, 'populationSize.nope'), undefined);
    assert.equal(valueOf(undefined, 'eliteCount'), undefined);
});

test('overriddenParameters mirrors the server for 4 to 10 parts', () => {
    assert.deepEqual([...overriddenParameters(3)], []);
    assert.deepEqual([...overriddenParameters(4)].sort(), ['eliteCount', 'populationSize.min']);
    assert.deepEqual([...overriddenParameters(10)].sort(), ['eliteCount', 'populationSize.min']);
    assert.deepEqual([...overriddenParameters(11)], []);
});

test('the start form lists every parameter; the Edit dialog the runtime ones', () => {
    setParameterRules(SERVER);
    const initial = getInitialParameters(defaultParameterValues(SERVER));
    assert.equal(initial.length, PARAMETER_DEFINITIONS.length);
    assert.equal(initial.find(i => i.key === 'populationSize.min')!.value, '100');
    assert.equal(getRuntimeParameters(undefined).length, 0);
    const running: SimulationRuntimeParameters = {
        populationSize: 400, eliteCount: 3, mutationRate: 0.07, crossoverRate: 0.75, maxEpochsCount: 20000,
        windowSize: 50, cycleCount: 10, tournamentSize: 5, repairWeight: 3, flipWeight: 1, donorFraction: 0.5,
    };
    const runtime = getRuntimeParameters(running);
    assert.deepEqual(runtime.map(i => i.key), PARAMETER_DEFINITIONS.filter(d => d.runtime).map(d => d.key));
    assert.ok(!runtime.some(i => i.key === 'windowSize' || i.key.startsWith('populationSize')));
});

test('getSimulationParameters sends every item as a number, dotted keys nested', () => {
    const items = getInitialParameters(defaultParameterValues(SERVER));
    const request = getSimulationParameters(items);
    assert.deepEqual(request.populationSize, {min: 100, max: 60000});
    assert.equal(request.mutationRate, 0.07);
    assert.equal(request.windowSize, 50);
    assert.equal(Object.keys(request).length, PARAMETER_DEFINITIONS.length - 1); // populationSize holds two
});

test('Simulation.validate checks the rules, then the relations', () => {
    setParameterRules(SERVER);
    const simulation = (values: object) => {
        const s = new Simulation({unitType: 'mm', partsData: [], sawWidth: '0'});
        s.initialParameters = values;
        return s.validate();
    };
    assert.deepEqual(simulation({}), []);
    assert.deepEqual(simulation({windowSize: 0, eliteCount: 0}), [
        'Elite count must be a whole number, at least 1.', 'Window size must be a whole number from 1 to 250.']);
    assert.deepEqual(simulation({populationSize: {min: 50, max: 40}}), ['Minimum population size is greater than Maximum population size.']);
    assert.deepEqual(simulation({populationSize: {min: 5, max: 40}, eliteCount: 6}), ['Elite count is greater than Minimum population size.']);
    assert.deepEqual(simulation({windowSize: 100, maxEpochsCount: 50}), ['Window size is greater than Number of generations.']);
});
