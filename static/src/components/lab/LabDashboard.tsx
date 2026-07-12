import React, {useMemo} from 'react';
import {PlayerPanel} from "./PlayerPanel";
import {CaptionPanel} from "./CaptionPanel";
import {TimeSeriesLineChart} from "./TimeSeriesLineChart";
import {SpreadScatterChart} from "./SpreadScatterChart";
import {LabControlPanel} from "./LabControlPanel";
import {LabStatisticsPanel} from "./LabStatisticsPanel";
import {ParametersPanel} from "./ParametersPanel";
import {BestSolutionPanel} from "./BestSolutionPanel";
import {SeriesDefinition, TimeSeriesMultiChart} from "./TimeSeriesMultiChart";
import {SeriesDefinition as VarSeriesDefinition, TimeSeriesVariationChart} from "./TimeSeriesVariationChart";
import * as echarts from "echarts";
import {DashboardState} from "../../services/status";
import {SimulationResults} from "../../services/requests";

interface LabDashboardProps {
    dashboardState: DashboardState;
    startTime?: Date;
    hasSheets: boolean;
    units: string;
    switchModeHandler: (isLab: boolean) => void;
    register: (id: string, chart?: echarts.ECharts) => void;
    onResultsHandler: (results: SimulationResults) => void;
}

export const LabDashboard: React.FC<LabDashboardProps> = ({dashboardState, startTime, units, hasSheets,
                                                              switchModeHandler, register, onResultsHandler}) => {

    const saturationSeries: SeriesDefinition[] = useMemo(() => [
        {id: 'SR', name: 'Saturation Rate', color: '#0B35B7'},
    ], []);

    const bestFitnessSeries: VarSeriesDefinition = useMemo(() => {
        return {id: 'BF', name: 'Best', color: '#0B35B7'};
    }, []);

    const avgFitnessSeries: VarSeriesDefinition = useMemo(() => {
        return {id: 'AF', name: 'Average', color: '#A60AC2'};
    }, []);

    const devFitnessSeries: VarSeriesDefinition = useMemo(() => {
        return {id: 'AFV', name: 'Deviation', color: '#EC9EFA', areaColor: 'rgba(236, 158, 250, 0.61)', stack: 'dev'};
    }, []);

    const avgEFitnessSeries: VarSeriesDefinition = useMemo(() => {
        return {id: 'EF', name: 'Average elite', color: '#470ac2'};
    }, []);

    const devEFitnessSeries: VarSeriesDefinition = useMemo(() => {
        return {id: 'EFV', name: 'Elite deviation', color: '#7c58da', areaColor: 'rgba(124,88,218,0.61)', stack: 'edev'};
    }, []);

    return (
        <div className="dashboard">
            <div className="dashboardSection">
                <div className="dashboardPanel">
                    <CaptionPanel text={'Simulation Lab'} />
                    <PlayerPanel dashboardState={dashboardState} onResultsHandler={onResultsHandler}/>
                </div>
                <div className="dashboardPanel">
                    <LabStatisticsPanel dashboardState={dashboardState} startTime={startTime} />
                </div>
                <div className="dashboardPanel">
                    <ParametersPanel dashboardState={dashboardState} />
                </div>
                <div className="dashboardPanel">
                    <BestSolutionPanel dashboardState={dashboardState} hasSheets={hasSheets} units={units} />
                </div>
            </div>
            <div className="dashboardSection">
                <div className="dashboardPanel">
                    <LabControlPanel
                        dashboardState={dashboardState}
                        onLabClickHandler={switchModeHandler}
                    />
                </div>
                <div className="dashboardPanel">
                    <TimeSeriesLineChart
                        id="lab-best-v2-chart"
                        caption={`Area over time, sq.${units}`}
                        width="100%"
                        height="45.5mm"
                        register={register}
                    />
                    {hasSheets && (
                        <TimeSeriesLineChart
                            id="lab-best-v1-chart"
                            caption="Sheets count over time"
                            width="100%"
                            height="45.5mm"
                            register={register}
                        />
                    )}
                </div>
            </div>
            <div className="dashboardSection">
                <div className={"dashboardPanel"}>
                    <TimeSeriesVariationChart
                        id="lab-epoch-series-chart"
                        bestSeries={bestFitnessSeries}
                        avgSeries={avgFitnessSeries}
                        devSeries={devFitnessSeries}
                        avgESeries={avgEFitnessSeries}
                        devESeries={devEFitnessSeries}
                        caption="Generation state"
                        width="100%"
                        height="73mm"
                        register={register}
                    />
                    <TimeSeriesVariationChart
                        id="lab-window-series-chart"
                        avgSeries={avgFitnessSeries}
                        devSeries={devFitnessSeries}
                        avgESeries={avgEFitnessSeries}
                        devESeries={devEFitnessSeries}
                        caption="Window state"
                        width="100%"
                        height="72.9mm"
                        register={register}
                    />
                </div>
            </div>
            <div className="dashboardSection">
                <div className={"dashboardPanel"}>
                    <TimeSeriesMultiChart
                        id="lab-saturation-chart"
                        series={saturationSeries}
                        caption="Saturation rate"
                        width="100%"
                        height="73mm"
                        register={register}
                    />
                    <SpreadScatterChart
                        id="lab-solution-vectors-chart"
                        caption="Solutions spread"
                        width="100%"
                        height="72.9mm"
                        xAxisName="Sheets"
                        yAxisName="Area"
                        register={register}
                    />
                </div>
            </div>
        </div>
    );
};