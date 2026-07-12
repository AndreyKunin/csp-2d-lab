import * as echarts from "echarts";

export class Adapter {
    id: string;
    chart?: echarts.ECharts;

    public reload() {

    }

    public setChart(chart?: echarts.ECharts) {
        this.chart = chart;
        this.reload();
    }

    constructor(id: string, chart?: echarts.ECharts) {
        this.id = id;
        this.chart = chart;
    }
}