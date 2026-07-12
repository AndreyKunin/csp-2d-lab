import * as echarts from "echarts";
import React, {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {LabDashboard} from "./LabDashboard";
import {MonitorDashboard} from "./MonitorDashboard";
import {DashboardState} from "../../services/status";
import {SimulationResults} from "../../services/requests";
import {ChartsMediator} from "../../services/charts";
import {SimulationStatus} from "../../utils/simulation";

interface DashboardProps {
    simulationId: string;
    startTime?: Date;
    initialStatus: SimulationStatus;
    initialLabMode: boolean;
    hasSheets: boolean;
    units: string;
    onError: (error: unknown) => void;
    onResultsHandler: (results: SimulationResults) => void;
}


export const Dashboard: React.FC<DashboardProps> = ({ simulationId, startTime, initialStatus, initialLabMode,
                                                        units, hasSheets, onError,
                                                        onResultsHandler }) => {
    const dashboardRef = useRef<HTMLDivElement>(null);

    const dashboardState = useMemo<DashboardState>(
        () => new DashboardState(simulationId, initialStatus, initialLabMode, onError)
        , [simulationId]);

    const chartsMediator = useMemo<ChartsMediator>(
        () => new ChartsMediator(dashboardState)
        , [dashboardState]);

    const [labMode, setLabMode] = useState(dashboardState.isLabMode());

    useEffect(() => {
        if (!dashboardState || !chartsMediator) {
            return;
        }
        if (dashboardRef.current) {
            dashboardRef.current.scrollIntoView({ behavior: 'smooth' });
        }
        const unsubscribe = dashboardState.onControlStateChange(() => {
            setLabMode(dashboardState.isLabMode());
        });
        return () => {
            unsubscribe();
            chartsMediator.dispose();
        }
    }, [dashboardState, chartsMediator]);

    const registerChart = useCallback((id: string, chart?: echarts.ECharts) => {
        chartsMediator.registerChart(id, chart);
    }, [simulationId, chartsMediator]);

    return (
        <div ref={dashboardRef}>
            {labMode && (
                <LabDashboard
                    dashboardState={dashboardState}
                    startTime={startTime}
                    hasSheets={hasSheets}
                    units={units}
                    switchModeHandler={(m: boolean) => dashboardState.setLabMode(m)}
                    register={registerChart}
                    onResultsHandler={onResultsHandler}
                />
            )}
            {!labMode && (
                <MonitorDashboard
                    dashboardState={dashboardState}
                    startTime={startTime}
                    hasSheets={hasSheets}
                    units={units}
                    switchModeHandler={(m: boolean) => dashboardState.setLabMode(m)}
                    register={registerChart}
                    onResultsHandler={onResultsHandler}
                />
            )}
        </div>
    );
};