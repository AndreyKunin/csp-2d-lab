import React, {useEffect, useMemo, useRef, useState} from 'react';
import {Solution} from '../utils/model';
import {FixedGrid} from "./FixedGrid";
import {ResultsGrid} from "./ResultsGrid";
import UpButton from "./UpButton";

interface ResultsProps {
  solutions: Solution[];
  selectedSolution: number | null;
  units: string;
  onSolutionSelect: (index: number) => void;
}

export const Results: React.FC<ResultsProps> = ({ solutions, selectedSolution, units, onSolutionSelect }) => {

  const resultsData = useMemo(() => {
      const data = solutions.map((solution, index) => (
          [
              `${index + 1}.`,
              solution.fitnessVector ? solution.fitnessVector[0].toString() : '',
              solution.fitnessVector ? (Math.round(solution.fitnessVector[1]*1000)/1000).toString(): '',
          ]
      ));

      const duplicates = new Set<string>();
      const results: string[][] = []
      for (const row of data) {
          const key = row[1] + ',' + row[2];
          if (!duplicates.has(key)) {
              results.push(row);
          }
          duplicates.add(key);
      }

      return results;
  }, [solutions]);

  useEffect(() => {
      if (resultsData.length === 1) {
          onSolutionSelect(0);
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
