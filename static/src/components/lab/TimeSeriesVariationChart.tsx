import React, {useCallback, useEffect, useRef} from 'react';
import * as echarts from 'echarts';
import {Adapter} from "../../services/adapter";

interface TimeSeriesVariationChartProps {
    id: string;
    bestSeries?: SeriesDefinition;
    avgSeries: SeriesDefinition;
    devSeries: SeriesDefinition;
    avgESeries: SeriesDefinition;
    devESeries: SeriesDefinition;
    caption: string;
    width: string;
    height: string;
    zoom?: boolean;
    register: (id: string, chart?: echarts.ECharts) => void;
}

export interface SeriesDefinition {
    id: string;
    name: string;
    color: string;
    areaColor?: string;
    stack?: string;
}

interface NamedDataPoint {
    name: string;
    value: [string, number];
}

interface SeriesEntry {
    id: string;
    data: NamedDataPoint[];
}

export interface DataPointStack {
    x: string;
    bestSeriesY: number | null;
    avgSeriesY: number;
    devSeriesY: number;
    avgESeriesY: number;
    devESeriesY: number;
}

export class TimeSeriesAdapter extends Adapter {
    bestSeriesId: string | null;
    avgSeriesId: string;
    devSeriesId: string;
    devSeriesHighId: string;
    devSeriesLowId: string;
    avgESeriesId: string;
    devESeriesId: string;
    devESeriesHighId: string;
    devESeriesLowId: string;
    maxItems: number;
    series: Map<string, SeriesEntry>;

    constructor(id: string,
                bestSeriesId: string | null,
                avgSeriesId: string, devSeriesId: string,
                avgESeriesId: string, devESeriesId: string,
                maxItems: number,
                chart?: echarts.ECharts) {
        super(id, chart);
        this.bestSeriesId = bestSeriesId;
        this.avgSeriesId = avgSeriesId;
        this.devSeriesId = devSeriesId;
        this.avgESeriesId = avgESeriesId;
        this.devESeriesId = devESeriesId;
        this.maxItems = maxItems;
        this.series = new Map<string, SeriesEntry>();
        this.devSeriesHighId = this.devSeriesId + 'H';
        this.devSeriesLowId = this.devSeriesId + 'L';
        this.devESeriesHighId = this.devESeriesId + 'H';
        this.devESeriesLowId = this.devESeriesId + 'L';
    }

    private lazyGetSeriesEntry(seriesId: string): SeriesEntry {
        let seriesEntry = this.series.get(seriesId);
        if (!seriesEntry) {
            seriesEntry = {
                id: seriesId,
                data: [],
            };
            this.series.set(seriesId, seriesEntry);
        }
        return seriesEntry;
    }

    public addData(...dataPoints: DataPointStack[]) {
        let avgSeriesEntry = this.lazyGetSeriesEntry(this.avgSeriesId);
        let devSeriesEntryLow = this.lazyGetSeriesEntry(this.devSeriesLowId);
        let devSeriesEntry = this.lazyGetSeriesEntry(this.devSeriesId);
        let devSeriesEntryHigh = this.lazyGetSeriesEntry(this.devSeriesHighId);
        let avgESeriesEntry = this.lazyGetSeriesEntry(this.avgESeriesId);
        let devESeriesEntryLow = this.lazyGetSeriesEntry(this.devESeriesLowId);
        let devESeriesEntry = this.lazyGetSeriesEntry(this.devESeriesId);
        let devESeriesEntryHigh = this.lazyGetSeriesEntry(this.devESeriesHighId);
        dataPoints.forEach(dataPoint => {
            if (this.bestSeriesId && dataPoint.bestSeriesY) {
                const bestSeriesEntry = this.lazyGetSeriesEntry(this.bestSeriesId);
                bestSeriesEntry.data.push({
                    name: dataPoint.x,
                    value: [dataPoint.x, dataPoint.bestSeriesY]
                });
            }
            avgSeriesEntry.data.push({
                name: dataPoint.x,
                value: [dataPoint.x, dataPoint.avgSeriesY]
            });
            const absDev = Math.abs(dataPoint.devSeriesY);
            devSeriesEntryLow.data.push({
                name: dataPoint.x,
                value: [dataPoint.x, dataPoint.avgSeriesY - absDev]
            });
            devSeriesEntry.data.push({
                name: dataPoint.x,
                value: [dataPoint.x, absDev + absDev]
            });
            devSeriesEntryHigh.data.push({
                name: dataPoint.x,
                value: [dataPoint.x, 0]
            });
            avgESeriesEntry.data.push({
                name: dataPoint.x,
                value: [dataPoint.x, dataPoint.avgESeriesY]
            });
            const absEDev = Math.abs(dataPoint.devESeriesY);
            devESeriesEntryLow.data.push({
                name: dataPoint.x,
                value: [dataPoint.x, dataPoint.avgESeriesY - absEDev]
            });
            devESeriesEntry.data.push({
                name: dataPoint.x,
                value: [dataPoint.x, absEDev + absEDev]
            });
            devESeriesEntryHigh.data.push({
                name: dataPoint.x,
                value: [dataPoint.x, 0]
            });
        });
        while (avgSeriesEntry.data.length > this.maxItems) {
            avgSeriesEntry.data.shift();
            devSeriesEntryLow.data.shift();
            devSeriesEntry.data.shift();
            devSeriesEntryHigh.data.shift();
            avgESeriesEntry.data.shift();
            devESeriesEntryLow.data.shift();
            devESeriesEntry.data.shift();
            devESeriesEntryHigh.data.shift();
        }
        if (this.bestSeriesId) {
            const bestSeriesEntry = this.lazyGetSeriesEntry(this.bestSeriesId);
            while (bestSeriesEntry.data.length > this.maxItems) {
                bestSeriesEntry.data.shift();
            }
        }
        this.reload();
    }

    public reload() {
        if (this.chart) {
            this.chart.setOption({
                series: Array.from(this.series.values())
            });
        }
    }
}

export const TimeSeriesVariationChart: React.FC<TimeSeriesVariationChartProps> = (
    {id, bestSeries, avgSeries, devSeries, avgESeries, devESeries, caption, width, height, zoom, register}) => {

    const chartRef = useRef<HTMLDivElement>(null);
    const chartInstance = useRef<echarts.ECharts | null>(null);

    const addSeries = useCallback((
        optionSeries: any[],
        legendData: any[] | null,
        isArea: boolean,
        showTooltip: boolean,
        series?: SeriesDefinition
    ) => {
        if (series) {
            let option: any;
            if (isArea) {
                option = {
                    id: series.id,
                    name: series.name,
                    type: 'line',
                    data: [],
                    areaStyle: {
                        color: series.areaColor,
                    },
                    lineStyle: {
                        opacity: 0
                    },
                    itemStyle: {
                        color: series.color,
                    },
                    symbol: 'none'
                }
            } else {
                option = {
                    id: series.id,
                    name: series.name,
                    data: [],
                    type: 'line',
                    symbol: 'none',
                    lineStyle: {
                        color: series.color,
                        width: 1,
                    },
                    symbolSize: 5,
                    itemStyle: {
                        color: series.color,
                        borderWidth: 1,
                    },
                };
            }
            if (series.stack) {
                option.stack = series.stack;
            }
            if (!showTooltip) {
                option.tooltip = {
                    show: false
                }
            }
            optionSeries.push(option);
            if (legendData) {
                legendData.push(series.name);
            }
        }
    }, []);

    useEffect(() => {
        if (chartRef.current) {
            chartInstance.current = echarts.init(chartRef.current);

            const option = {
                renderer: 'canvas',
                useDirtyRect: true,
                tooltip: {
                    trigger: 'axis',
                    showContent: true,
                    backgroundColor: '#f6edb2',
                    borderColor: '#2c2a24',
                    borderWidth: 1,
                    padding: [6, 9],
                    textStyle: {
                        color: '#2c2a24',
                        fontSize: 10,
                        fontWeight: 'normal'
                    },
                    formatter: function (params: any) {
                        let tooltipContent = '';
                        params.forEach(function (item: any) {
                            let value = item.value[1];
                            if (item.seriesId === devSeries.id) {
                                value = +(value / 2).toFixed(3);
                            }
                            tooltipContent += item.marker + ' ' + value + '<br/>';
                        });
                        return tooltipContent;
                    }
                },
                legend: {
                    data: [],
                    bottom: 10,
                    textStyle: {
                        color: '#2c2a24',
                    },
                    inactiveColor: '#2c2a24',
                    lineStyle: {
                        width: 2,
                        inactiveColor: '#f6edb2',
                        opacity: 1
                    },
                    itemStyle: {
                        opacity: 0,
                    }
                },
                grid: {
                    top: 20,
                    left: 20,
                    bottom: 60,
                    containLabel: true,
                },
                xAxis: {
                    type: 'time',
                    axisLabel: {
                        formatter: '{mm}:{ss}',
                        hideOverlap: true
                    },
                    axisLine: {
                        show: true
                    },
                },
                yAxis: {
                    type: 'value',
                    axisTick: {
                        show: true
                    },
                    axisLine: {
                        show: true
                    },
                    splitLine: {
                        show: false
                    },
                    axisLabel: {
                        hideOverlap: true
                    },
                },
                series: []
            };

            if (zoom) {
                // @ts-ignore
                option.toolbox = {
                    right: 0,
                    bottom: 0,
                    orient: 'vertical',
                    iconStyle: {
                        borderColor: '#2c2a24',
                    },
                    itemGap: 2,
                    feature: {
                        dataZoom: {
                            yAxisIndex: 'none',
                            iconStyle: {
                                borderColor: '#2c2a24',
                            },
                            title: {
                                zoom: 'Zoom',
                                back: 'Restore'
                            }
                        }
                    }
                };
            }

            const devSeriesHighName = devSeries.name + ' (high)';
            const devSeriesLowName = devSeries.name + ' (low)';
            const devESeriesHighName = devESeries.name + ' (high)';
            const devESeriesLowName = devESeries.name + ' (low)';

            const optionSeries: any[] = [];
            const legendData: any[] = [];
            addSeries(optionSeries, legendData, false, true, bestSeries);
            addSeries(optionSeries, legendData, false, true, avgSeries);
            addSeries(optionSeries, null, false, false, {
                id: devSeries.id + 'L',
                name: devSeriesLowName,
                color: devSeries.color,
                stack: devSeries.stack,
            });
            addSeries(optionSeries, legendData, true, true, {
                id: devSeries.id,
                name: devSeries.name,
                color: devSeries.color,
                areaColor: devSeries.areaColor,
                stack: devSeries.stack,
            });
            addSeries(optionSeries, null, false, false, {
                id: devSeries.id + 'H',
                name: devSeriesHighName,
                color: devSeries.color,
                stack: devSeries.stack,
            });
            addSeries(optionSeries, legendData, false, true, avgESeries);
            addSeries(optionSeries, null, false, false, {
                id: devESeries.id + 'L',
                name: devESeriesLowName,
                color: devESeries.color,
                stack: devESeries.stack,
            });
            addSeries(optionSeries, legendData, true, true, {
                id: devESeries.id,
                name: devESeries.name,
                color: devESeries.color,
                areaColor: devESeries.areaColor,
                stack: devESeries.stack,
            });
            addSeries(optionSeries, null, false, false, {
                id: devESeries.id + 'H',
                name: devESeriesHighName,
                color: devESeries.color,
                stack: devESeries.stack,
            });
            // @ts-ignore
            option.series = optionSeries;
            // @ts-ignore
            option.legend.data = legendData;

            chartInstance.current.setOption(option);
            chartInstance.current.on('legendselectchanged', args => {
                const params = args as any;
                if (params.name === devSeries.name) {
                    const isSelected = params.selected[devSeries.name];
                    if (chartInstance.current) {
                        const eventType = isSelected ? 'legendSelect' : 'legendUnSelect';
                        chartInstance.current.dispatchAction({
                            type: eventType,
                            name: devSeriesLowName
                        });
                        chartInstance.current.dispatchAction({
                            type: eventType,
                            name: devSeriesHighName
                        });
                    }
                }
            });
            register(id, chartInstance.current);

            const handleResize = () => {
                chartInstance.current?.resize();
            };

            window.addEventListener('resize', handleResize);

            return () => {
                window.removeEventListener('resize', handleResize);
                register(id, undefined);
                chartInstance.current?.dispose();
            };
        }
    }, [addSeries]);


    return (
        <div className="graphPanel">
            <div>{caption}</div>
            <div style={{ width, height }}>
                <div
                    ref={chartRef}
                    style={{ width: '100%', height: '100%' }}
                    className="timeSeriesChart"
                />
            </div>
        </div>
    );
};
