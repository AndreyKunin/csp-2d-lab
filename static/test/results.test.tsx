// FE-01: the results list hides solutions with the same [sheets, area] as an
// earlier one, but the selection must still name a solution of the full list
// (App, Visualizer and getTree index model.solutions with it).

import {test} from 'node:test';
import * as assert from 'node:assert/strict';
import {renderToStaticMarkup} from 'react-dom/server';
import {Results, resultRows} from '../src/components/Results';
import {Solution} from '../src/utils/model';

// A and A2 share [sheets, area]: A2 is hidden. B is shown as row "3.".
const solutions: Solution[] = [
    {sheets: [{number: 0, tree: '1 2 S'}], fitnessVector: [1, 5000, 1]},
    {sheets: [{number: 0, tree: '2 1 S'}], fitnessVector: [1, 5000, 1]},
    {sheets: [{number: 0, tree: '1 2 L'}], fitnessVector: [2, 9000, 1]},
];

// rowsOf renders the list and returns each row's label ("1.", "3.") and
// whether it is highlighted as the selected one.
function rowsOf(selectedSolution: number | null): { label: string, selected: boolean }[] {
    const html = renderToStaticMarkup(
        <Results solutions={solutions} selectedSolution={selectedSolution} units="mm" onSolutionSelect={() => {}}/>);
    const rows: { label: string, selected: boolean }[] = [];
    const rowPattern = /<div( class="([^"]*)")?><div>(\d+\.)<\/div>/g;
    for (let m = rowPattern.exec(html); m; m = rowPattern.exec(html)) {
        rows.push({label: m[3], selected: m[2] === 'selectedLayout'});
    }
    return rows;
}

test('the list shows the first of equal solutions, numbered by their place in the full list', () => {
    assert.deepEqual(rowsOf(null).map(r => r.label), ['1.', '3.']);
});

test('the selected solution is highlighted by its index in the full list', () => {
    assert.deepEqual(rowsOf(0).filter(r => r.selected).map(r => r.label), ['1.']);
    assert.deepEqual(rowsOf(2).filter(r => r.selected).map(r => r.label), ['3.'], 'solution 2 is row "3."');
});

test('a hidden solution highlights no row', () => {
    assert.deepEqual(rowsOf(1).filter(r => r.selected).map(r => r.label), [], 'solution 1 is hidden, not row "3."');
});

test('each row carries the index of its solution in the full list, which a click selects', () => {
    assert.deepEqual(resultRows(solutions), [
        {index: 0, cells: ['1.', '1', '5000']},
        {index: 2, cells: ['3.', '2', '9000']},
    ]);
    assert.deepEqual(resultRows([]), []);
    // Solutions without a fitness vector share the empty key: only the first is shown.
    assert.deepEqual(resultRows([{sheets: []}, {sheets: []}]).map(r => r.index), [0]);
});
