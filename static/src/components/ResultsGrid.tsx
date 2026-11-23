import React, {useEffect, useState} from 'react';

// ResultRow is one row of the results list: its cells and the index of its
// solution in the full list of solutions (FE-01: rows of equal solutions are
// hidden, so the row's position is not the solution's index).
export interface ResultRow {
    index: number;
    cells: string[];
}

interface ResultsGridProps {
    id: string;
    units: string;
    data: ResultRow[];
    selectedSolution: number | null;
    onSolutionSelect: (index: number) => void;
}

export const ResultsGrid: React.FC<ResultsGridProps> = (
    ({ id, units, data, selectedSolution, onSolutionSelect }: ResultsGridProps) => {

    useEffect(() => {
        if (data.length > 0) {
            onSolutionSelect(data[0].index);
        }
    }, [data]);

    return (
        <div className="resultsGrid" id={id}>
            <div>
                <div>Layout</div>
                <div>Sheets</div>
                <div>Area, {units}<sup>2</sup></div>
            </div>
            {data.map(row => (
                <div key={id+row.index} className={selectedSolution === row.index ? 'selectedLayout' : ''}>
                    {row.cells.map((cell, colIndex) => (
                        <div
                            key={id+row.index+"c"+colIndex}
                            onClick={() => onSolutionSelect(row.index)}
                        >
                            {cell}
                        </div>
                    ))}
                </div>
            ))}
        </div>
    );
});