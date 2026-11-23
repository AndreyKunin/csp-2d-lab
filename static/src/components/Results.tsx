import React, {useEffect, useMemo, useRef, useState} from 'react';
import {Solution} from '../utils/model';
import {FixedGrid} from "./FixedGrid";
import {ResultsGrid} from "./ResultsGrid";
import UpButton from "./UpButton";
import {ResultRow} from "./ResultsGrid";

// resultRows lists the solutions to show: the first of the solutions with the
// same [sheets, area], each with its index in the full list (FE-01), which the
// selection uses (App, Visualizer and getTree index model.solutions with it).
export function resultRows(solutions: Solution[]): ResultRow[] {
    const seen = new Set<string>();
    const rows: ResultRow[] = [];
    solutions.forEach((solution, index) => {
        const sheets = solution.fitnessVector ? solution.fitnessVector[0].toString() : '';
        const area = solution.fitnessVector ? (Math.round(solution.fitnessVector[1] * 1000) / 1000).toString() : '';
        const key = sheets + ',' + area;
        if (!seen.has(key)) {
            rows.push({index, cells: [`${index + 1}.`, sheets, area]});
        }
        seen.add(key);
    });
    return rows;
}

interface ResultsProps {
  solutions: Solution[];
  selectedSolution: number | null;
  units: string;
  onSolutionSelect: (index: number) => void;
}

export const Results: React.FC<ResultsProps> = ({ solutions, selectedSolution, units, onSolutionSelect }) => {

  const resultsData = useMemo(() => resultRows(solutions), [solutions]);

  useEffect(() => {
      if (resultsData.length === 1) {
          onSolutionSelect(resultsData[0].index);
      }
  }, [resultsData]);

  return (
    <>
        {resultsData.length > 1 && (
            <div className="solutionsList">
                <div>
                    <span className="sectionHeader">Best layouts</span>
                </div>
                <ResultsGrid
                    id="results_grid"
                    units={units}
                    data={resultsData}
                    selectedSolution={selectedSolution}
                    onSolutionSelect={onSolutionSelect}
                />
                <div className="sectionSpacer">&nbsp;</div>
                <div className="actions">
                    <UpButton className="fileButton" />
                </div>
            </div>
        )}
    </>
  );
};
