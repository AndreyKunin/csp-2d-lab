import React, {useEffect, useRef} from 'react';
import * as echarts from 'echarts';
import {Adapter} from "../../services/adapter";

interface TimeSeriesLineChartProps {
    id: string;
    caption: string;
    width: string;
    height: string;
    zoom?: boolean;
    register: (id: string, chart?: echarts.ECharts) => void;
}

export interface DataPoint {
    x: string;
    y: number;
}

interface NamedDataPoint {
    name: string;
    value: [string, number];
}

export class TimeSeriesAdapter extends Adapter {
    maxItems: number;
    data: NamedDataPoint[];

    constructor(id: string, maxItems: number, chart?: echarts.ECharts) {
        super(id, chart);
        this.maxItems = maxItems;
        this.data = [];
    }

    public addData(...dataPoints: DataPoint[]) {
        dataPoints.forEach(dataPoint => {
            this.data.push({
                name: dataPoint.x,
                value: [dataPoint.x, dataPoint.y]
            });
        });
        while (this.data.length > this.maxItems) {
            this.data.shift();
        }
        this.reload();
    }

    public reload() {
        if (this.chart) {
            this.chart.setOption({
                series: [{
                    data: this.data
                }]
            });
        }
    }
}

export const TimeSeriesLineChart: React.FC<TimeSeriesLineChartProps> = (
    {id, caption, width, height, zoom, register}) => {
    const chartRef = useRef<HTMLDivElement>(null);
    const chartInstance = useRef<echarts.ECharts | null>(null);

    useEffect(() => {
        if (chartRef.current) {
            chartInstance.current = echarts.init(chartRef.current);

            const option = {
                renderer: 'canvas',
                useDirtyRect: true,
                tooltip: {
                    trigger: 'item',
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
                        return  '' + params.value[1];
                    }
                },

                grid: {
                    top: 20,
                    bottom: 40,
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
                series: [
                    {
                        data: [],
                        type: 'line',
                        smooth: !zoom,
                        symbol: zoom ? 'none' : 'circle',
                        lineStyle: {
                            color: '#2c2a24',
                            width: !!zoom ? 1 : 2,
                        },
                        symbolSize: 5,
                        itemStyle: {
                            color: '#2c2a24',
                            borderWidth: 1,
                        },
                    }
                ]
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
    }, [zoom]);


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
