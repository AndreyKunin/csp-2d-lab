// FE-07: joinRects, getTree and computeLayout on the shared geometry vectors
// (test/testdata/trees/*.json, also checked by the Go tests through
// test/support/oracle), and the sheet handling of getTree and Layout.

import {test} from 'node:test';
import * as assert from 'node:assert/strict';
import {getTree, RectNode} from '../src/utils/rect';
import {computeLayout, Layout} from '../src/utils/layout';
import {parseSize} from '../src/utils/numbers';
import {checkPlacement, contentOf, EPS, leafCount, loadRepoJson, makeModel, num, setLayout} from './helpers';

interface TreeVector {
    name: string;
    kerf: number;
    parts: { [label: string]: [number, number] };
    tree: string;
    long: number;
    short: number;
}

const vectors = loadRepoJson<TreeVector[]>('test/testdata/trees/basic.json');

test('the shared tree vectors are there', () => {
    assert.ok(vectors.length >= 10);
    assert.ok(vectors.some(v => v.name.startsWith('S-tie')), 'no tie case (GA-12)');
});

for (const v of vectors) {
    test(`shared tree vector: ${v.name}`, () => {
        // Labels are 1..k: the model's rows in label order.
        const labels = Object.keys(v.parts).sort((a, b) => Number(a) - Number(b));
        labels.forEach((label, i) => assert.equal(label, String(i + 1), 'labels must be the row numbers'));
        const model = makeModel('mm', labels.map(l => [String(v.parts[l][0]), String(v.parts[l][1]), '1']), null, String(v.kerf));
        setLayout(model, [v.tree]);
        const root = getTree(model, 0, 0, parseSize(model.sawWidth, model.fraction));
        assert.equal(root.getWidth(), v.long, `${v.name}: long side`);
        assert.equal(root.getHeight(), v.short, `${v.name}: short side`);
        const sizeOf = (label: string): [number, number] => v.parts[label];
        assert.deepEqual(checkPlacement(root, computeLayout(root, 1, 0, 0), sizeOf, leafCount(v.tree)), []);
    });
}

test('joinRects keeps exact fractions and normalises the long side first', () => {
    const a = new RectNode('1', parseSize('1 1/4', true), parseSize('3/8', true), 0, true);
    const b = new RectNode('2', parseSize('3/8', true), parseSize('1/2', true), 0, true);
    const s = new RectNode('S', 0, 0, 0);
    s.joinRects(a, b, parseSize('1/16', true));
    // S: b (1/2 x 3/8) is rotated? a.short 3/8 >= b.long 1/2 is false; b.short 3/8 >= a.long 5/4 is false:
    // no rotation, long = 5/4 + 1/2 + 1/16 = 29/16, short = max(3/8, 3/8) = 3/8.
    assert.equal(s.formatDimensions(), '1 13/16 x 3/8');
    assert.ok(s.isFraction);
    const l = new RectNode('L', 0, 0, 0);
    l.joinRects(a, b, 0);
    // L: long = max(5/4, 1/2) = 5/4, short = 3/8 + 3/8 = 3/4.
    assert.equal(l.formatDimensions(), '1 1/4 x 3/4');
});

test('getTree rejects a malformed tree', () => {
    const model = makeModel('mm', [['10', '20', '2']], null, '0');
    setLayout(model, ['1 S']);
    assert.throws(() => getTree(model, 0, 0, 0), /Invalid layout/);
});

// The sheet as entered: the UI keeps width and length as typed, and either can
// be the long side. The drawn sheet is the sheet, whichever way it was typed.
for (const sheet of [['1288', '752'], ['752', '1288']]) {
    test(`getTree fills a ${sheet.join(' x ')} sheet with waste`, () => {
        const model = makeModel('mm', [['500', '300', '2'], ['200', '100', '2']], sheet, '0');
        // 1 1 L = 600 x 500; 2 2 S = 400 x 100, rotated across the 500 side (500 >= 400): 700 x 500.
        setLayout(model, ['1 1 L 2 2 S S']);
        const root = getTree(model, 0, 0, 0);
        assert.equal(contentOf(root).formatDimensions(), '700 x 500');
        assert.equal(root.formatDimensions(), '1288 x 752');
        const sizeOf = (label: string): [number, number] => label === '1' ? [500, 300] : [200, 100];
        assert.deepEqual(checkPlacement(root, computeLayout(root, 1, 0, 0), sizeOf, 4), []);
    });
}

test('Layout scales to the sheet whichever way it was entered', () => {
    const canvas = {getBoundingClientRect: () => ({width: 1000, height: 600})} as unknown as HTMLCanvasElement;
    const rect = new RectNode('1', 1288, 752, 0);
    const scale = (sheetWidth: number, sheetLength: number) =>
        (new Layout(canvas, 'mm', sheetWidth, sheetLength) as unknown as { rescale(r: RectNode): number }).rescale(rect);
    // The long side is drawn horizontally: min((950-19)/1288, (570-19)/752).
    const want = Math.min((Math.floor(1000 * 0.95) - 19) / 1288, (Math.floor(600 * 0.95) - 19) / 752);
    assert.ok(Math.abs(scale(1288, 752) - want) < EPS);
    assert.ok(Math.abs(scale(752, 1288) - want) < EPS, `entered as 752 x 1288: scale ${scale(752, 1288)}, want ${want}`);
    assert.ok(num(rect.width) >= num(rect.height));
});
