// FE-07: the start form's input model (src/utils/model.ts).

import {test} from 'node:test';
import * as assert from 'node:assert/strict';
import {makeModel} from './helpers';

const four = [['10', '20', '4']];

test('Model numbers the rows and detects fractions', () => {
    const m = makeModel('in', [['1 1/4', '3', '2'], ['2', '5/8', '3']], ['48', '96'], '1/8');
    assert.deepEqual(m.parts.map(p => p.label), ['1', '2']);
    assert.equal(m.fraction, true);
    assert.equal(makeModel('mm', four, null, '3/32').fraction, false); // the saw width does not switch the model
    assert.deepEqual(m.getNumericParts(true).map(p => [p.width, p.length, p.count]), [[1.25, 3, 2], [2, 0.625, 3]]);
    assert.deepEqual(m.getNumericSheets(true), {width: 48, length: 96});
    assert.equal(makeModel('mm', four, null, '0').getNumericSheets(true), undefined);
});

test('a valid input has no errors', () => {
    assert.deepEqual(makeModel('mm', four, ['100', '100'], '0').validate(), []);
    assert.deepEqual(makeModel('mm', [['10', '20', '60'], ['5', '5', '40']], null, '0').validate(), []);
});

test('no parts', () => {
    assert.deepEqual(makeModel('mm', [], null, '0').validate(), ['Please specify parts.']);
});

test('the total part count is from 4 to 100', () => {
    assert.match(makeModel('mm', [['10', '20', '3']], null, '0').validate().join(), /less than 4/);
    assert.match(makeModel('mm', [['10', '20', '60'], ['5', '5', '41']], null, '0').validate().join(), /limited to 100\. Requested: 101/);
});

test('duplicate dimensions, either way round', () => {
    const errors = makeModel('mm', [['10', '20', '2'], ['20', '10', '2']], null, '0').validate();
    assert.deepEqual(errors, ['The parts # 1 and # 2 (20 x 10) are duplicates.']);
});

test('every part fits the sheet, whichever way the sheet was entered', () => {
    for (const sheet of [['100', '50'], ['50', '100']]) {
        assert.deepEqual(makeModel('mm', [['40', '90', '4']], sheet, '0').validate(), [], sheet.join(' x '));
        assert.match(makeModel('mm', [['60', '90', '4']], sheet, '0').validate().join(), /does not fit the given sheet size/, sheet.join(' x '));
        assert.match(makeModel('mm', [['10', '120', '4']], sheet, '0').validate().join(), /does not fit/, sheet.join(' x '));
    }
});

test('the precision is at most 1/1000', () => {
    assert.match(makeModel('mm', [['10.0001', '20', '4']], null, '0').validate().join(), /precision/);
    assert.deepEqual(makeModel('mm', [['10.001', '20', '4']], null, '0').validate(), []);
    assert.match(makeModel('in', [['1 1/1024', '2', '4']], null, '0').validate().join(), /precision of 1\/1024/);
});

test('the sizes span at most 4 orders of magnitude', () => {
    assert.match(makeModel('mm', [['1', '1', '2'], ['20000', '20000', '2']], null, '0').validate().join(), /more than 4 orders of magnitude/);
    assert.deepEqual(makeModel('mm', [['1', '1', '2'], ['10000', '9000', '2']], null, '0').validate(), []);
});
