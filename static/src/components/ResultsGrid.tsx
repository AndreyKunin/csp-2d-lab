import React, {useEffect, useState} from 'react';

interface ResultsGridProps {
    id: string;
    units: string;
    data: string[][];
    selectedSolution: number | null;
    onSolutionSelect: (index: number) => void;
}

export const ResultsGrid: React.FC<ResultsGridProps> = (
    ({ id, units, data, selectedSolution, onSolutionSelect }: ResultsGridProps) => {

    useEffect(() => {
        if (data.length > 0) {
            onSolutionSelect(0);
        }
    }, [data]);

    return (
        <div className="resultsGrid" id={id}>
            <div>
                <div>Layout</div>
                <div>Sheets</div>
                <div>Area, {units}<sup>2</sup></div>
            </div>
            {data.map((row, rowIndex) => (
                <div key={id+rowIndex} className={selectedSolution === rowIndex ? 'selectedLayout' : ''}>
                    {row.map((cell, colIndex) => (
                        <div
                            key={id+rowIndex+"c"+colIndex}
                            onClick={() => onSolutionSelect(rowIndex)}
                        >
                            {cell}
                        </div>
                    ))}
                </div>
            ))}
        </div>
    );
});