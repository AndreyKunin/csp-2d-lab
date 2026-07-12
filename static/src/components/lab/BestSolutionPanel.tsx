import React, {useEffect, useMemo, useState} from 'react';
import {ProblemInsights, SimulationStatistics} from "../../utils/simulation";
import {getFillPattern} from "../../utils/numbers";
import {DashboardState} from "../../services/status";

interface BestSolutionPanelProps {
    dashboardState: DashboardState;
    hasSheets: boolean;
    units: string;
}

export const BestSolutionPanel: React.FC<BestSolutionPanelProps> = ({ dashboardState, units, hasSheets }) => {
    const [problem, setProblem] = useState(dashboardState.getProblem());

    useEffect(() => {
        const unsubscribe = dashboardState.onStatusChange(() => {
            setProblem(dashboardState.getProblem());
        });
        return () => unsubscribe();
    }, [dashboardState]);

    const sheets = useMemo(() =>
            problem && problem.bestFitnessVector ? '' + problem.bestFitnessVector[0] : ''
        , [problem]
    );

    const area = useMemo(() =>
            problem && problem.bestFitnessVector ? problem.bestFitnessVector[1].toFixed(2) : ''
        , [problem]
    );

    const sheetsPattern = useMemo(() => getFillPattern(sheets), [sheets]);
    const areaPattern = useMemo(() => getFillPattern(area), [area]);


    return (
        <div className="solutionVectorPanel">
            <div>Best solution</div>
            <div>
                {hasSheets && (
                    <div>Sheets</div>
                )}
                <div>Area</div>
            </div>
            <div className="ledDisplay">
                {hasSheets && (
                    <div className="ledMono" style={{'--digit-pattern': `"${sheetsPattern}"`} as React.CSSProperties}>
                        {sheets}
                    </div>
                )}
                <div className="ledMono" style={{'--digit-pattern': `"${areaPattern}"`} as React.CSSProperties}>
                    {area}
                </div>
            </div>
            <div>
                {hasSheets && (
                    <div>&nbsp;</div>
                )}
                <div>{units}<span className="superscript">2</span></div>
            </div>
        </div>
    );
};