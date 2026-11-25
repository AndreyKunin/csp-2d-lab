// Shared helpers of the frontend unit tests (npm test).

import * as fs from 'fs';
import * as path from 'path';
import * as math from 'mathjs';
import {Model} from '../src/utils/model';
import {RectNode} from '../src/utils/rect';
import {RectView} from '../src/utils/layout';

// The repository root, from static/test-build/test/.
const REPO_ROOT = path.resolve(__dirname, '..', '..', '..');

export function loadRepoJson<T>(relative: string): T {
    return JSON.parse(fs.readFileSync(path.join(REPO_ROOT, relative), 'utf-8')) as T;
}

// makeModel builds the UI's Model exactly as the start form does: part rows
// [width, length, count] (labels are the row numbers), the sheet as entered
// [width, length] or none, and the saw width as entered.
export function makeModel(unit: 'mm' | 'in', parts: string[][], sheet: string[] | null, sawWidth: string): Model {
    return new Model({unitType: unit, partsData: parts, sheetsData: sheet ? [sheet] : undefined, sawWidth});
}

// setLayout gives the model one solution whose sheets have the given trees, as
// the result endpoint sends them.
export function setLayout(model: Model, trees: string[]): void {
    model.solutions = [{sheets: trees.map((tree, number) => ({number, tree}))}];
}

export const num = (v: math.Fraction | number): number => math.number(v as math.Fraction) as number;

export const EPS = 1e-9;

// leafCount is the number of parts in a tree string.
export function leafCount(tree: string): number {
    return tree.split(' ').filter(token => token !== 'S' && token !== 'L').length;
}

// contentOf returns the packed parts of a sheet tree: getTree joins a faux
// (waste) rect to fill the sheet; the content is its other child.
export function contentOf(root: RectNode): RectNode {
    return root.right?.isFaux ? root.left! : root;
}

// checkPlacement verifies a placement (computeLayout at scale 1, origin 0):
// every part has its own size in one orientation, lies inside the root, and no
// two parts overlap; the faux rects lie inside the root and overlap no part.
// It returns the problems found.
export function checkPlacement(root: RectNode, placed: { sliceRects: RectView[], partRects: RectView[] },
                               sizeOf: (label: string) => [number, number], expectedParts: number): string[] {
    const problems: string[] = [];
    const W = root.getWidth(), H = root.getHeight();
    const inside = (r: RectView) => r.x >= -EPS && r.y >= -EPS && r.x + r.w <= W + EPS && r.y + r.h <= H + EPS;
    const overlap = (a: RectView, b: RectView) =>
        Math.min(a.x + a.w, b.x + b.w) - Math.max(a.x, b.x) > EPS && Math.min(a.y + a.h, b.y + b.h) - Math.max(a.y, b.y) > EPS;
    const show = (r: RectView) => `${r.slice.rect.name} at (${r.x}, ${r.y}) ${r.w} x ${r.h}`;

    if (placed.partRects.length !== expectedParts) {
        problems.push(`${placed.partRects.length} parts placed, want ${expectedParts}`);
    }
    for (const r of placed.partRects) {
        const [a, b] = sizeOf(r.slice.rect.name);
        const sameSize = (Math.abs(r.w - a) < EPS && Math.abs(r.h - b) < EPS) || (Math.abs(r.w - b) < EPS && Math.abs(r.h - a) < EPS);
        if (!sameSize) {
            problems.push(`part ${show(r)} is not ${a} x ${b}`);
        }
        if (!inside(r)) {
            problems.push(`part ${show(r)} is outside the ${W} x ${H} sheet`);
        }
    }
    for (let i = 0; i < placed.partRects.length; i++) {
        for (let j = i + 1; j < placed.partRects.length; j++) {
            if (overlap(placed.partRects[i], placed.partRects[j])) {
                problems.push(`parts ${show(placed.partRects[i])} and ${show(placed.partRects[j])} overlap`);
            }
        }
    }
    for (const f of placed.sliceRects.filter(r => r.slice.rect.isFaux)) {
        if (!inside(f)) {
            problems.push(`waste ${show(f)} is outside the ${W} x ${H} sheet`);
        }
        for (const p of placed.partRects) {
            if (overlap(f, p)) {
                problems.push(`waste ${show(f)} overlaps part ${show(p)}`);
            }
        }
    }
    return problems;
}
