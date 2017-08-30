import { Injectable } from '@angular/core';
import { Http, URLSearchParams } from '@angular/http';
import { Dimension, Graph } from '../model';

@Injectable()
export class ReportingService {

    constructor(private http: Http) {
        // do something
    }

    public async getServiceDimensions(search?: URLSearchParams): Promise<Dimension> {
        let s = search || new URLSearchParams();
        return await this.http.get(`/dimensions/services?${s.toString()}`).map((res) => new Dimension(res.json())).toPromise();
    }

    public async getRegionDimensions(search?: URLSearchParams): Promise<Dimension> {
        let s = search || new URLSearchParams();
        return await this.http.get(`/dimensions/regions?${s.toString()}`).map((res) => new Dimension(res.json())).toPromise();
    }

    public async getAccountDimensions(search?: URLSearchParams): Promise<Dimension> {
        let s = search || new URLSearchParams();
        return await this.http.get(`/dimensions/accounts?${s.toString()}`).map((res) => new Dimension(res.json())).toPromise();
    }

    public async getResourceTags(search?: URLSearchParams): Promise<Dimension> {
        let s = search || new URLSearchParams();
        return await this.http.get(`/dimensions/resourcetags?${s.toString()}`).map((res) => new Dimension(res.json())).toPromise();
    }

    public async runReport(search?: URLSearchParams, resources?: boolean): Promise<Graph[]> {
        let path = resources ? 'count' : 'cost';
        let s: URLSearchParams = search || new URLSearchParams();
        return await this.http.get(`/${path}?${s.toString()}`).map((res) => {
            let data = res.json().data;
            if (!data) { // null condition
                data = [];
            }
            let graphData: Array<Graph> = new Array<Graph>();
            for (let i = 0; i < data.length; i++) {
                graphData.push(new Graph(data[i]));
            }
            return graphData;
        }).toPromise();
    }

    public async runUsageReport(usageMetric: string, service: string, search?: URLSearchParams): Promise<Graph[]> {
        let s: URLSearchParams = search || new URLSearchParams();
        return await this.http.get(`/usage/${service}/${usageMetric}?${s.toString()}`).map((res) => {
            let data = res.json().data;
            if (!data) { // null condition
                data = [];
            }
            let graphData: Array<Graph> = new Array<Graph>();
            for (let i = 0; i < data.length; i++) {
                graphData.push(new Graph(data[i]));
            }
            return graphData;
        }).toPromise();
    }

    public async runReportWithoutInterval(search?: URLSearchParams, resources?: boolean): Promise<Graph[]> {
        let s: URLSearchParams = search || new URLSearchParams();
        if (s.paramsMap.has('interval')) {
            s.paramsMap.delete('interval');
        }
        return await this.runReport(search, resources);
    }

    public generateSummary(val: Graph[], countFlag): any {
        let graph: any[] = new Array<any>();
        let total: number = 0;
        for (let i = 0; i < val.length; i++) {
            let t = val[i].getSumOfAllValues();
            let dVal = countFlag ? t : (t).toFixed(2);
            let slice = [val[i].getSeriesDisplayName(), dVal, val[i].tags];
            graph.push(slice);
            total = total + t;
        }
        graph.sort((a, b) => {
            return b[1] - a[1];
        });
        return {
            total,
            graph,
        };
    }

    public sortData(val: Graph[]): Graph[] {
        let data: Graph[] = [];
        let items: any[] = new Array<any>();
        for (let i = 0; i < val.length; i++) {
            let t = val[i].getSumOfAllValues();
            let slice = [i, t];
            items.push(slice);
        }

        items.sort((a, b) => {
            return b[1] - a[1];
        });

        for (let j = 0; j < items.length; j++) {
            data.push(val[items[j][0]]);
        }
        return data;
    }

    public async saveAccountName(reportId: string, accountId: string, data: any): Promise<any> {
        return await this.http.put(`/reports/${reportId}/accounts/${accountId}`, data).toPromise();
    }
}
