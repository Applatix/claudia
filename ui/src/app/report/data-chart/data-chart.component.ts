import { Component, Input, ElementRef, OnInit, ViewEncapsulation } from '@angular/core';
import { Graph } from '../../model';
import * as c3 from 'c3';
import * as moment from 'moment';

import { ReportingService } from '../../services';
import { resourceMetricKey, DefaultMetrics } from '../config';

@Component({
    selector: 'axc-data-chart',
    encapsulation: ViewEncapsulation.None,
    templateUrl: './data-chart.component.html',
    styles: [
        require('./data-chart.scss'),
    ],
})
export class DataChartComponent implements OnInit {

    @Input()
    set data(val: any) {
        this._data = this.reportingService.sortData(val.graphData);

        if (val.isCountData && val.resourceSummary) {
            this._summary = val.resourceSummary;
            this.isCountMetric = val.isCountData;
        } else {
            this._summary = null;
            this.isCountMetric = false;
        }
        if (this.ready) {
            this.renderGraph();
        }
    }

    @Input()
    set intervalMetric(val: string) {
        if (val === '1h') {
            this.titleDateFormat = this.hourlIntervalTitleTimeFormat;
        } else {
            this.titleDateFormat = this.defaultTitleTimeFormat;
        }
    }

    @Input()
    public name: string;

    @Input()
    set type(val: string) {
        this._type = val;
        if (this.ready) {
            this.renderGraph();
        }
    }

    @Input()
    set group(val: string) {
        this._grouping = val;
    }
    @Input()
    set metric(val: string) {
        this._metric = val;
        if (DefaultMetrics.indexOf(val) > -1) {
            this.metricSymbol = '$';
            if (resourceMetricKey === this._metric) {
                this.metricSymbol = '';
            }
        } else {
            this.metricSymbol = '';
        }
    }

    private defaultTitleTimeFormat = 'YYYY-MM-DD';
    private hourlIntervalTitleTimeFormat = 'YYYY-MM-DD HH:mm';
    private titleDateFormat = this.defaultTitleTimeFormat;
    private ready: boolean = false;
    private target;
    private _summary;
    private _data: any;
    private _grouping: string;
    private _metric: string;
    private metricSymbol: string = '';
    private isCountMetric: boolean = false;
    private _type: string = 'DONUT'; // valid types - DONUT, LINE, STACKED
    private breakdown: Array<any> = Array<any>();
    private lineBreakdown: any = {};
    private stackedBreakdown: any = {};
    private chart: c3.ChartAPI;
    private count: number = 10;

    constructor(
        private el: ElementRef,
        private reportingService: ReportingService) {
    }

    public ngOnInit() {
        this.target = $(this.el.nativeElement).find('.the-donut');
        this.ready = true;
        if (this._data) {
            this.renderGraph();
        }
    }

    private renderGraph() {
        switch (this._type) {
            case 'DONUT':
                this.renderDonut();
                break;
            case 'LINE':
                this.renderLineGraph();
                break;
            case 'STACKED':
                this.renderStackedGraph();
                break;
            default:
                console.error('Type of chart needs to be supplied');
        }
    }

    private renderStackedGraph() {
        this.updateBreakDownForStackedGraph(this._data);
        this.chart = c3.generate({
            bindto: this.target[0],
            data: this.stackedBreakdown,
            tooltip: {
                format: {
                    title: (x) => {
                        return moment(x).format(this.titleDateFormat);
                    },
                    value: (val, ratio, id, index) => { return this.metricSymbol + parseFloat(val.toFixed(2)); },
                },
            },
            axis: {
                y: {
                    label: {
                        text: this._metric + (this.metricSymbol),
                        position: 'outer-middle',
                    },
                },
                x: {
                    label: {
                        text: 'Time',
                        position: 'outer-right',
                    },
                    type: 'timeseries',
                    tick: {
                        format: '%Y-%m-%d',
                    },
                },
            },
        });

    }

    private updateBreakDownForStackedGraph(val: Graph[]) {
        let firstData = val[0].values;
        let timeAxis = ['x'];
        let cols = [];
        let grpList = [];
        for (let i = 0; i < firstData.length; i++) {
            timeAxis.push(new Date(firstData[i][0]).toISOString());
        }
        cols.push(timeAxis);

        let counter = val.length === this.count ? this.count : (this.count - 1);
        let otherSum = 0;
        let slice: any[];
        let otherVals = [];
        let firstTime = true;

        for (let j = 0; j < val.length; j++) {
            let name = val[j].getSeriesDisplayName();
            slice = [name];

            for (let k = 0; k < val[j].values.length; k++) {
                let v = val[j].values[k][1];
                if (counter > 0) {
                    slice.push(v);
                } else {
                    if (firstTime && otherVals.length < val[j].values.length) {
                        otherVals.push(v);
                    } else {
                        otherVals[k] += v;
                    }
                }
                if (otherVals.length === val[j].values.length) {
                    firstTime = false;
                }
            }

            if (counter > 0) {
                cols.push(slice);
                grpList.push(name);
                counter--;
            }
        }

        slice = ['Others'];
        slice = slice.concat(otherVals);
        cols.push(slice);
        grpList.push('Others');

        this.stackedBreakdown = {
            x: 'x',
            xFormat: '%Y-%m-%dT%H:%M:%S.%LZ',
            columns: cols,
            type: 'bar',
            groups: [grpList],
        };
    }
    private renderLineGraph() {
        this.updateBreakDownForLineGraph(this._data);

        this.chart = c3.generate({
            bindto: this.target[0],
            data: this.lineBreakdown,
            tooltip: {
                format: {
                    title: (x) => {
                        return moment(x).format(this.titleDateFormat);
                    },
                    value: (val, ratio, id, index) => { return this.metricSymbol + parseFloat(val.toFixed(2)); },
                },
            },
            axis: {
                y: {
                    label: {
                        text: this._metric + (this.metricSymbol),
                        position: 'outer-middle',
                    },
                },
                x: {
                    label: {
                        text: 'Time',
                        position: 'outer-right',
                    },
                    type: 'timeseries',
                    tick: {
                        format: '%Y-%m-%d',
                    },
                },
            },
        });


        this.chart.flush();
    }
    private updateBreakDownForLineGraph(val: Graph[]) {

        let firstData = val[0].values;
        let timeAxis = ['x'];
        let cols = [];
        for (let i = 0; i < firstData.length; i++) {
            timeAxis.push(new Date(firstData[i][0]).toISOString());
        }
        cols.push(timeAxis);
        let counter = val.length === this.count ? this.count : (this.count - 1);
        let otherSum = 0;
        let slice: any[];
        let otherVals = [];
        let firstTime = true;

        for (let j = 0; j < val.length; j++) {
            let name = val[j].getSeriesDisplayName();
            slice = [name];

            for (let k = 0; k < val[j].values.length; k++) {
                let v = val[j].values[k][1];
                if (counter > 0) {
                    slice.push(v);
                } else {
                    if (firstTime && otherVals.length < val[j].values.length) {
                        otherVals.push(v);
                    } else {
                        otherVals[k] += v;
                    }
                }
                if (otherVals.length === val[j].values.length) {
                    firstTime = false;
                }
            }

            if (counter > 0) {
                cols.push(slice);
                counter--;
            }
        }

        slice = ['Others'];
        slice = slice.concat(otherVals);
        cols.push(slice);

        this.lineBreakdown = {
            x: 'x',
            xFormat: '%Y-%m-%dT%H:%M:%S.%LZ',
            columns: cols,
        };
    }

    private renderDonut() {
        if (!this.isCountMetric) {
            this.updateBreakDownForDonut(this._data);
        } else {
            this.updateBreakDownForDonut(this._summary);
        }
        this.chart = c3.generate({
            bindto: this.target[0],
            data: {
                columns: this.breakdown,
                type: 'donut',
                onclick: (d, i) => {
                    // do nothing
                },
                onmouseover: (d, i) => {
                    // do nothing
                },
                onmouseout: (d, i) => {
                    // do nothing
                },
            },
            donut: {
                title: this._metric,
            },
        });

    }

    private updateBreakDownForDonut(val: Graph[]) {
        let graphingData = [];
        let counter = val.length === this.count ? this.count : (this.count - 1);
        let otherSum = 0;
        for (let i = 0; i < val.length; i++) {
            let v = val[i].getSumOfAllValues();
            if (counter > 0) {
                let slice = [val[i].getSeriesDisplayName(), v];
                if (v > 0) {
                    graphingData.push(slice);
                }
                counter--;
            } else {
                otherSum += v;
            }
        }
        if (otherSum > 0) {
            let otherSlice = ['Others', otherSum];
            graphingData.push(otherSlice);
        }

        this.breakdown = graphingData;
    }


}
