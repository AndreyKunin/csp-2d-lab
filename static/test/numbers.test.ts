// FE-07: sizes as the user types them (src/utils/numbers.ts).

import {test} from 'node:test';
import * as assert from 'node:assert/strict';
import * as math from 'mathjs';
import {formatSize, getFillPattern, isFraction, parseSize} from '../src/utils/numbers';

test('isFraction recognises a fraction or a mixed fraction', () => {
    assert.equal(isFraction('3/32'), true);
    assert.equal(isFraction('1 1/4'), true);
    assert.equal(isFraction('12.5'), false);
    assert.equal(isFraction('12'), false);
});

test('parseSize reads decimals, fractions and mixed fractions', () => {
    assert.equal(parseSize('12', false), 12);
    assert.equal(parseSize('12.5', false), 12.5);
    assert.equal(parseSize('3/32', false), 0.09375);
    assert.equal(parseSize('1 1/4', false), 1.25);
    assert.equal(parseSize(' 2  3/8 ', false), 2.375);
    // In fraction mode the value stays exact.
    assert.ok(math.equal(parseSize('1 1/4', true), math.fraction(5, 4)));
    assert.ok(math.equal(parseSize('1/3', true), math.fraction(1, 3)));
    assert.ok(math.equal(parseSize('2', true), math.fraction(2)));
    assert.equal((parseSize('1/3', true) as math.Fraction).d, 3);
});

test('formatSize writes whole numbers, mixed fractions and decimals', () => {
    assert.equal(formatSize(math.fraction(5, 4), true), '1 1/4');
    assert.equal(formatSize(math.fraction(3, 32), true), '3/32');
    assert.equal(formatSize(math.fraction(2), true), '2');
    assert.equal(formatSize(math.fraction(29, 16), true), '1 13/16');
    assert.equal(formatSize(12.5, false), '12.5');
    assert.equal(formatSize(1288, false), '1288');
});

test('a size survives a format and parse round trip', () => {
    for (const text of ['1 1/4', '3/32', '17', '5 15/16']) {
        assert.equal(formatSize(parseSize(text, true), true), text);
    }
    for (const text of ['12.5', '0.75', '1288']) {
        assert.equal(formatSize(parseSize(text, false), false), text);
    }
});

test('getFillPattern masks digits', () => {
    assert.equal(getFillPattern('1 3/16'), '8 8 88');
});
