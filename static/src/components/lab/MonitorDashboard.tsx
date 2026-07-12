import React from 'react';
import {ProblemInsights, SimulationStatistics, SimulationStatus} from "../../utils/simulation";
import {PlayerPanel} from "./PlayerPanel";
import {CaptionPanel} from "./CaptionPanel";
import {MonitorStatisticsPanel} from "./MonitorStatisticsPanel";
import {BestSolutionPanel} from "./BestSolutionPanel";
import {TimeSeriesLineChart} from "./TimeSeriesLineChart";
import {SpreadScatterChart} from "./SpreadScatterChart";
import {LabControlPanel} from "./LabControlPanel";
import {MonitorControlPanel} from "./MonitorControlPanel";
import * as echarts from "echarts";
import {DashboardState} from "../../services/status";
import {SimulationResults} from "../../services/requests";

interface MonitorDashboardProps {
    dashboardState: DashboardState;
    startTime?: Date;
    hasSheets: boolean;
    units: string;
    switchModeHandler: (isLab: boolean) => void;
    register: (id: string, chart?: echarts.ECharts) => void;
    onResultsHandler: (results: SimulationResults) => void;
}

export const MonitorDashboard: React.FC<MonitorDashboardProps> = ({dashboardState, startTime, units,
                                                                      hasSheets, switchModeHandler, register, onResultsHandler }) => {

    return (
        <div className="dashboard">
            <div className="dashboardSection">
                <div className="dashboardPanel">
                    <CaptionPanel text={'Simulation Monitor'} />
                    <PlayerPanel dashboardState={dashboardState} onResultsHandler={onResultsHandler}/>
                </div>
                <div className="dashboardPanel">
                    <MonitorStatisticsPanel  dashboardState={dashboardState} startTime={startTime} />
                </div>
                <div className="dashboardPanel">
                    <BestSolutionPanel dashboardState={dashboardState} hasSheets={hasSheets} units={units} />
                </div>
            </div>
            <div className="dashboardSection">
                <div className="dashboardPanel">
                    <MonitorControlPanel onLabClickHandler={switchModeHandler} />
                </div>
                <div className="dashboardPanel">
                    <TimeSeriesLineChart
                        id="monitor-best-fitness-chart"
                        caption="Best fitness series"
                        width="100%"
                        height="56.8mm"
                        register={register}
                    />
                </div>
            </div>
            {hasSheets && (
                <div className="dashboardSection">
                    <div className="dashboardPanel">
                        <SpreadScatterChart
                            id="monitor-solution-vectors-chart"
                            caption="Solutions spread"
                            width="100%"
                            height="82.3mm"
                            xAxisName="Sheets"
                            yAxisName="Area"
                            register={register}
                        />
                    </div>
                </div>
            )}
        </div>
    );
};