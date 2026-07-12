import React, {useEffect, useRef} from 'react';
import * as echarts from 'echarts';
import {Adapter} from "../../services/adapter";

interface SpreadScatterChartProps {
    id: string;
    caption: string;
    xAxisName: string;
    yAxisName: string;
    width: string;
    height: string;
    register: (id: string, chart?: echarts.ECharts) => void;
}

export interface DataPoint {
    x: number;
    y: number;
}

interface NamedDataPoint {
    name: string;
    value: [number, number];
}

export class ScatterAdapter extends Adapter {
    data: NamedDataPoint[];

    constructor(id: string, chart?: echarts.ECharts) {
        super(id, chart);
        this.data = [];
    }

    public setData(dataPoints: DataPoint[]) {
        const newData: NamedDataPoint[] = [];
        for (let i = 0; i < dataPoints.length; ++i) {
            const dataPoint = dataPoints[i];
            newData.push({
                name: '' + dataPoint.x + '_' + dataPoint.y,
                value: [dataPoint.x, dataPoint.y]
            });
        }
        this.data = newData;
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

export const SpreadScatterChart: React.FC<SpreadScatterChartProps> = ({id, caption, width, height, xAxisName, yAxisName, register}) => {
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
                        return  `(${params.value[0]}, ${params.value[1]})`;
                    }
                },

                grid: {
                    top: 20,
                    bottom: 40,
                },
                xAxis: {
                    name: xAxisName,
                    nameLocation: 'center',
                    nameTextStyle: {
                        fontSize: 12
                    },
                    axisLabel: {
                        hideOverlap: true
                    },
                    splitLine: {
                        show: true,
                        lineStyle: {
                            color: '#2c2a24',
                            width: 1,
                            type: 'dashed'
                        }
                    }
                },
                yAxis: {
                    name: yAxisName,
                    nameLocation: 'middle',
                    nameTextStyle: {
                        fontSize: 12
                    },
                    splitLine: {
                        show: true,
                        lineStyle: {
                            color: '#2c2a24',
                            width: 1,
                            type: 'dashed'
                        }
                    }
                },
                series: [
                    {
                        data: [],
                        type: 'scatter',
                        lineStyle: {
                            color: '#2c2a24'
                        },
                        symbolSize: 4,
                        itemStyle: {
                            color: '#2c2a24',
                            borderColor: '#2c2a24',
                            borderWidth: 2,
                        },
                    }
                ]
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
    }, []);


    return (
        <div className="graphPanel">
            <div>{caption}</div>
            <div style={{ width, height }}>
                <div
                    ref={chartRef}
                    style={{ width: '100%', height: '100%' }}
                    className="spreadScatterChart"
                />
            </div>
        </div>
    );
};
