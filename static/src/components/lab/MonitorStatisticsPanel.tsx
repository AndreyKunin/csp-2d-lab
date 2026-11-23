import React, {useEffect, useMemo, useState} from 'react';
import {getFillPattern} from "../../utils/numbers";
import {StatusDisplay} from "./StatusDisplay";
import {DashboardState} from "../../services/status";

interface MonitorStatisticsPanelProps {
    dashboardState: DashboardState;
    startTime?: Date;
}

export const MonitorStatisticsPanel: React.FC<MonitorStatisticsPanelProps> = ({ dashboardState, startTime }) => {
    const [status, setStatus] = useState(dashboardState.getStatus());
    const [state, setState] = useState(dashboardState.getStatistics()?.state);
    const [parameters, setParameters] = useState(dashboardState.getStatistics()?.parameters);

    // Recomputed on every status update: the subscriber replaces state each poll
    // (with startTime alone, Duration kept its first value, FE-05).
    const duration = useMemo(() =>
        startTime ? new Date(Date.now() - startTime.valueOf()).toISOString().substring(11, 19) : ''
        , [startTime, state]
    );

    useEffect(() => {
        const unsubscribe = dashboardState.onStatusChange(() => {
            setStatus(dashboardState.getStatus());
            setState(dashboardState.getStatistics()?.state);
            setParameters(dashboardState.getStatistics()?.parameters);
        });
        return () => unsubscribe();
    }, [dashboardState]);

    const percentCompleted = useMemo(() => {
        if (parameters && state) {
            return '' + (+(state.epochState.epochNumber / parameters.maxEpochsCount * 100).toFixed(2));
        }
        return '';
    }, [parameters, state]);

    const durationPattern = useMemo(() => getFillPattern(duration), [duration]);
    const percentCompletedPattern = useMemo(() => getFillPattern(percentCompleted), [percentCompleted]);

    return (
        <div className="dashboardStatsPanel">
            <div>
                <div>Status</div>
                <div>Duration</div>
                <div>Progress</div>
            </div>
            <div className="ledDisplay">
                {status && <StatusDisplay status={status} />}
                <div className="ledMono" style={{'--digit-pattern': `"${durationPattern}"`} as React.CSSProperties}>
                    {duration}
                </div>
                <div className="ledMono" style={{'--digit-pattern': `"${percentCompletedPattern}"`} as React.CSSProperties}>
                    {percentCompleted}
                </div>
            </div>
            <div>
                <div>&nbsp;</div>
                <div>&nbsp;</div>
                <div>%</div>
            </div>
        </div>
    );
};