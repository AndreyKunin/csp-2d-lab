import React, {useEffect, useRef} from 'react';
import * as echarts from 'echarts';
import {Adapter} from "../../services/adapter";

interface TimeSeriesMultiChartProps {
    id: string;
    series: SeriesDefinition[];
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
}

export interface DataPointStack {
    x: string;
    ys: number[];
    ids: string[];
}

interface NamedDataPoint {
    name: string;
    value: [string, number];
}

interface SeriesEntry {
    id: string;
    data: NamedDataPoint[];
}

export class TimeSeriesAdapter extends Adapter {
    maxItems: number;
    series: Map<string, SeriesEntry>;

    constructor(id: string, maxItems: number, chart?: echarts.ECharts) {
        super(id, chart);
        this.maxItems = maxItems;
        this.series = new Map<string, SeriesEntry>();
    }

    public addData(...dataPoints: DataPointStack[]) {
        dataPoints.forEach(dataPoint => {
            for (let i = 0; i < dataPoint.ids.length; ++i) {
                let seriesEntry = this.series.get(dataPoint.ids[i]);
                if (!seriesEntry) {
                    seriesEntry = {
                        id: dataPoint.ids[i],
                        data: [],
                    };
                    this.series.set(dataPoint.ids[i], seriesEntry);
                }
                seriesEntry.data.push({
                    name: dataPoint.x,
                    value: [dataPoint.x, dataPoint.ys[i]]
                });
            }
        });
        for (const seriesEntry of this.series.values()) {
            while (seriesEntry.data.length > this.maxItems) {
                seriesEntry.data.shift();
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

export const TimeSeriesMultiChart: React.FC<TimeSeriesMultiChartProps> = (
    {id, series, caption, width, height, zoom, register}) => {
    const chartRef = useRef<HTMLDivElement>(null);
    const chartInstance = useRef<echarts.ECharts | null>(null);

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
                            tooltipContent += item.marker + ' ' + item.value[1] + '<br/>';
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

            const optionSeries = [];
            const legendData = []
            for (let i = 0; i < series.length; ++i) {
                const seriesDef = series[i];
                optionSeries[i] = {
                    id: seriesDef.id,
                    name: seriesDef.name,
                    data: [],
                    type: 'line',
                    symbol: 'none',
                    lineStyle: {
                        color: seriesDef.color,
                        width: 1,
                    },
                    symbolSize: 5,
                    itemStyle: {
                        color: seriesDef.color,
                        borderWidth: 1,
                    },
                }
                legendData[i] = seriesDef.name;
            }
            // @ts-ignore
            option.series = optionSeries;
            // @ts-ignore
            option.legend.data = legendData;

            chartInstance.current.setOption(option);
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
    }, []);


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
