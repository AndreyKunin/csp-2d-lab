import React, {useEffect, useMemo, useState} from 'react';
import {getFillPattern} from "../../utils/numbers";
import {DashboardState} from "../../services/status";

interface ParametersPanelProps {
    dashboardState: DashboardState;
}

export const ParametersPanel: React.FC<ParametersPanelProps> = ({ dashboardState }) => {
    const [parameters, setParameters] = useState(dashboardState.getStatistics()?.parameters);

    useEffect(() => {
        const unsubscribe = dashboardState.onStatusChange(() => {
            setParameters(dashboardState.getStatistics()?.parameters);
        });
        return () => unsubscribe();
    }, [dashboardState]);

    const minRate = useMemo(() => {
        if (parameters) {
            return '' + (+parameters.minRate.toFixed(3));
        }
        return '';
    }, [parameters]);

    const maxRate = useMemo(() => {
        if (parameters) {
            return '' + (+parameters.maxRate.toFixed(3));
        }
        return '';
    }, [parameters]);

    const mutationRate = useMemo(() => {
        if (parameters) {
            return '' + (+parameters.mutationRate.toFixed(2));
        }
        return '';
    }, [parameters]);

    const eliteCount = useMemo(() => {
        if (parameters) {
            return '' + parameters.eliteCount;
        }
        return '';
    }, [parameters]);

    const windowSize = useMemo(() => {
        if (parameters) {
            return '' + parameters.windowSize;
        }
        return '';
    }, [parameters]);

    const populationSize = useMemo(() => {
        if (parameters) {
            return '' + parameters.populationSize;
        }
        return '';
    }, [parameters]);

    const minRatePattern = useMemo(() => getFillPattern(minRate), [minRate]);
    const maxRatePattern = useMemo(() => getFillPattern(maxRate), [maxRate]);
    const mutationRatePattern = useMemo(() => getFillPattern(mutationRate), [mutationRate]);
    const eliteCountPattern = useMemo(() => getFillPattern(eliteCount), [eliteCount]);
    const windowSizePattern = useMemo(() => getFillPattern(windowSize), [windowSize]);
    const populationSizePattern = useMemo(() => getFillPattern(populationSize), [populationSize]);

    return (
        <div className="solutionVectorPanel">
            <div>Run time parameters</div>
            <div style={{flex: 1.3}}>
                <div>Min rate</div>
                <div>Max rate</div>
                <div>Mutation rate</div>
                <div>Elite count</div>
                <div>Window size</div>
                <div>Population</div>
            </div>
            <div className="ledDisplay" style={{flex: 1.7}}>
                <div className="ledMono" style={{'--digit-pattern': `"${minRatePattern}"`} as React.CSSProperties}>
                    {minRate}
                </div>
                <div className="ledMono" style={{'--digit-pattern': `"${maxRatePattern}"`} as React.CSSProperties}>
                    {maxRate}
                </div>
                <div className="ledMono" style={{'--digit-pattern': `"${mutationRatePattern}"`} as React.CSSProperties}>
                    {mutationRate}
                </div>
                <div className="ledMono" style={{'--digit-pattern': `"${eliteCountPattern}"`} as React.CSSProperties}>
                    {eliteCount}
                </div>
                <div className="ledMono" style={{'--digit-pattern': `"${windowSizePattern}"`} as React.CSSProperties}>
                    {windowSize}
                </div>
                <div className="ledMono" style={{'--digit-pattern': `"${populationSizePattern}"`} as React.CSSProperties}>
                    {populationSize}
                </div>
            </div>
            <div style={{flex: 0.5}}>
                <div>&nbsp;</div>
                <div>&nbsp;</div>
                <div>&nbsp;</div>
                <div>&nbsp;</div>
                <div>&nbsp;</div>
                <div>&nbsp;</div>
            </div>
        </div>
    );
};