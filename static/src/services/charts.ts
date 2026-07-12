import * as echarts from "echarts";
import {DashboardState} from "./status";
import {Adapter} from "./adapter";
import {DataPoint as VectorDataPoint, ScatterAdapter} from "../components/lab/SpreadScatterChart";
import {
    DataPointStack as VariationDataPointStack,
    TimeSeriesAdapter as TimeSeriesVariationAdapter
} from "../components/lab/TimeSeriesVariationChart";
import {
    DataPointStack as ImprovementDataPointStack,
    TimeSeriesAdapter as TimeSeriesMultiAdapter
} from "../components/lab/TimeSeriesMultiChart";
import {DataPoint as FitnessDataPoint, TimeSeriesAdapter} from "../components/lab/TimeSeriesLineChart";
import {SimulationJobFullStatus} from "./requests";
import {SimulationStatistics} from "../utils/simulation";


export class ChartsMediator {
    private callbacks: Set<() => void> = new Set<() => void>();

    private spreadScatterCharts: Map<string, ScatterAdapter> = new Map<string, ScatterAdapter>();
    private timeSeriesVariationCharts: Map<string, TimeSeriesVariationAdapter> = new Map<string, TimeSeriesVariationAdapter>();
    private timeSeriesMultiCharts: Map<string, TimeSeriesMultiAdapter> = new Map<string, TimeSeriesMultiAdapter>();
    private timeSeriesLineCharts: Map<string, TimeSeriesAdapter> = new Map<string, TimeSeriesAdapter>();
    private allCharts: Map<string, Adapter> = new Map<string, Adapter>();

    constructor(dashboardState: DashboardState) {
        this.spreadScatterCharts.set("lab-solution-vectors-chart",
            new ScatterAdapter("lab-solution-vectors-chart")
        );
        this.spreadScatterCharts.set("monitor-solution-vectors-chart",
            new ScatterAdapter("monitor-solution-vectors-chart")
        );
        this.timeSeriesLineCharts.set("lab-best-v1-chart",
            new TimeSeriesAdapter("lab-best-v1-chart", dashboardState.getDataPoints())
        );
        this.timeSeriesLineCharts.set("lab-best-v2-chart",
            new TimeSeriesAdapter("lab-best-v2-chart", dashboardState.getDataPoints())
        );
        this.timeSeriesLineCharts.set("monitor-best-fitness-chart",
            new TimeSeriesAdapter("monitor-best-fitness-chart", dashboardState.getDataPoints())
        );
        this.timeSeriesLineCharts.set("lab-elitism-chart",
            new TimeSeriesAdapter("lab-elitism-chart", dashboardState.getDataPoints())
        );
        this.timeSeriesLineCharts.set("lab-crossover-chart",
            new TimeSeriesAdapter("lab-crossover-chart", dashboardState.getDataPoints())
        );
        this.timeSeriesLineCharts.set("lab-mutation-chart",
            new TimeSeriesAdapter("lab-mutation-chart", dashboardState.getDataPoints())
        );
        this.timeSeriesMultiCharts.set("lab-saturation-chart",
            new TimeSeriesMultiAdapter("lab-saturation-chart", dashboardState.getDataPoints())
        );
        this.timeSeriesVariationCharts.set("lab-epoch-series-chart",
            new TimeSeriesVariationAdapter("lab-epoch-series-chart", 'BF', 'AF', 'AFV', 'EF', 'EFV', dashboardState.getDataPoints())
        );
        this.timeSeriesVariationCharts.set("lab-window-series-chart",
            new TimeSeriesVariationAdapter("lab-window-series-chart", null, 'AF', 'AFV', 'EF', 'EFV', dashboardState.getDataPoints())
        );
        this.spreadScatterCharts.forEach((value, key) => this.allCharts.set(key, value));
        this.timeSeriesVariationCharts.forEach((value, key) => this.allCharts.set(key, value));
        this.timeSeriesMultiCharts.forEach((value, key) => this.allCharts.set(key, value));
        this.timeSeriesLineCharts.forEach((value, key) => this.allCharts.set(key, value));


        this.callbacks.add(dashboardState.onControlStateChange(() => {
            for (const adapter of this.timeSeriesVariationCharts.values()) {
                adapter.maxItems = dashboardState.getDataPoints();
            }
            for (const adapter of this.timeSeriesMultiCharts.values()) {
                adapter.maxItems = dashboardState.getDataPoints();
            }
            for (const adapter of this.timeSeriesLineCharts.values()) {
                adapter.maxItems = dashboardState.getDataPoints();
            }
        }));

        this.callbacks.add(dashboardState.onStatusChange(() => {
            const jobStatus = dashboardState.getJobStatus();
            if (jobStatus) {
                this.updateRuntimeData(jobStatus);
            }
        }));
    }

    public dispose() {
        this.callbacks.forEach(callback => callback());
    }

    public registerChart(id: string, chart?: echarts.ECharts) {
        if (this.allCharts.has(id)) {
            this.allCharts.get(id)!.setChart(chart);
        }
    }

    private updateRuntimeData(status: SimulationJobFullStatus) {
        this.timeSeriesLineCharts.get("lab-best-v1-chart")?.addData(this.getFitness(status, 0));
        this.timeSeriesLineCharts.get("lab-best-v2-chart")?.addData(this.getFitness(status, 1));
        this.timeSeriesMultiCharts.get("lab-saturation-chart")?.addData(this.getSaturationRate(status.timestamp, status.statistics));
        this.timeSeriesVariationCharts.get("lab-epoch-series-chart")?.addData(this.getEpoch(status.timestamp, status.statistics));
        this.timeSeriesVariationCharts.get("lab-window-series-chart")?.addData(this.getWindow(status.timestamp, status.statistics));
        this.spreadScatterCharts.get("lab-solution-vectors-chart")?.setData(this.getSelectedVectors(status));
        this.timeSeriesLineCharts.get("monitor-best-fitness-chart")?.addData(this.getCompoundFitness(status));
        this.spreadScatterCharts.get("monitor-solution-vectors-chart")?.setData(this.getSelectedVectors(status));
    }

    private getFitness(status: SimulationJobFullStatus, index: number): FitnessDataPoint {
        if (!status.problem) {
            return {x: '', y: 0} as FitnessDataPoint;
        }
        return this._getFitness(status.timestamp, status.problem.bestFitnessVector || [], index);
    };

    private _getFitness(timestamp: Date, bestFitnessVector: number[], index: number): FitnessDataPoint {
        return {
            x: this.formatChartDatePoint(timestamp),
            y: bestFitnessVector[index] ?? 0
        };
    };

    private getSaturationRate(timestamp: Date, statistics?: SimulationStatistics): ImprovementDataPointStack {
        return {
            ids: ['SR'],
            x: this.formatChartDatePoint(timestamp),
            ys: [
                statistics?.state?.windowState.saturationRate ?? 0,
            ],
        }
    }

    private getEpoch(timestamp: Date, statistics?: SimulationStatistics): VariationDataPointStack {
        return {
            x: this.formatChartDatePoint(timestamp),
            bestSeriesY: statistics?.state?.epochState.bestFitness ?? 0,
            avgSeriesY: statistics?.state?.epochState.averageFitness ?? 0,
            devSeriesY: statistics?.state?.epochState.fitnessDeviation ?? 0,
            avgESeriesY: statistics?.state?.epochState.averageEliteFitness ?? 0,
            devESeriesY: statistics?.state?.epochState.eliteFitnessDeviation ?? 0,
        }
    }

    private getWindow(timestamp: Date, statistics?: SimulationStatistics): VariationDataPointStack  {
        return {
            x: this.formatChartDatePoint(timestamp),
            bestSeriesY: null,
            avgSeriesY: statistics?.state?.windowState.averageFitness ?? 0,
            devSeriesY: statistics?.state?.windowState.averageFitnessDeviation ?? 0,
            avgESeriesY: statistics?.state?.epochState.averageEliteFitness ?? 0,
            devESeriesY: statistics?.state?.epochState.eliteFitnessDeviation ?? 0,
        }
    }

    private getSelectedVectors(status: SimulationJobFullStatus): VectorDataPoint[] {
        const points: VectorDataPoint[] = [];
        status.problem?.selectedSpecies?.forEach(fitnessVector => {
            points.push({
                x: fitnessVector[0],
                y: fitnessVector[1]
            })
        });

        return points;
    };

    private getCompoundFitness(status: SimulationJobFullStatus): FitnessDataPoint {
        return {
            x: this.formatChartDatePoint(status.timestamp),
            y: status.statistics?.state?.epochState?.bestFitness ?? 0
        };
    };

    private formatChartDatePoint(date: Date): string {
        return date.toISOString()
            .replace('T', ' ')
            .replace(/\.\d{3}Z$/, '')
            .slice(0, 19)
    }
}