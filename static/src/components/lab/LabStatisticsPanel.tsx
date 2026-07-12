import React, {useEffect, useMemo, useState} from 'react';
import {getFillPattern} from "../../utils/numbers";
import {StatusDisplay} from "./StatusDisplay";
import {DashboardState} from "../../services/status";

interface LabStatisticsPanelProps {
    startTime?: Date;
    dashboardState: DashboardState;
}

export const LabStatisticsPanel: React.FC<LabStatisticsPanelProps> = ({ dashboardState, startTime }) => {
    const [status, setStatus] = useState(dashboardState.getStatus());
    const [state, setState] = useState(dashboardState.getStatistics()?.state);
    const [parameters, setParameters] = useState(dashboardState.getStatistics()?.parameters);

    const duration = useMemo(() =>
        startTime ? new Date(Date.now() - startTime.valueOf()).toISOString().substring(11, 19) : ''
        , [startTime]
    );

    useEffect(() => {
        const unsubscribe = dashboardState.onStatusChange(() => {
            setStatus(dashboardState.getStatus());
            setState(dashboardState.getStatistics()?.state);
            setParameters(dashboardState.getStatistics()?.parameters);
        });
        return () => unsubscribe();
    }, [dashboardState]);

    const epochNumber = useMemo(() => {
        if (state) {
            return '' + state.epochState.epochNumber;
        }
        return '';
    }, [state]);

    const epochsCount = useMemo(() => {
        if (parameters) {
            return '' + parameters.maxEpochsCount;
        }
        return '';
    }, [parameters]);

    const durationPattern = useMemo(() => getFillPattern(duration), [duration]);
    const epochNumberPattern = useMemo(() => getFillPattern(epochNumber), [epochNumber]);
    const epochsCountPattern = useMemo(() => getFillPattern(epochsCount), [epochsCount]);

    return (
        <div className="dashboardStatsPanel">
            <div>
                <div>Status</div>
                <div>Duration</div>
                <div>Generation #</div>
                <div>Generations</div>
            </div>
            <div className="ledDisplay">
                {status && <StatusDisplay status={status} />}
                <div className="ledMono" style={{'--digit-pattern': `"${durationPattern}"`} as React.CSSProperties}>
                    {duration}
                </div>
                <div className="ledMono" style={{'--digit-pattern': `"${epochNumberPattern}"`} as React.CSSProperties}>
                    {epochNumber}
                </div>
                <div className="ledMono" style={{'--digit-pattern': `"${epochsCountPattern}"`} as React.CSSProperties}>
                    {epochsCount}
                </div>
            </div>
            <div>
                <div>&nbsp;</div>
                <div>&nbsp;</div>
                <div>&nbsp;</div>
                <div>&nbsp;</div>
            </div>
        </div>
    );
};