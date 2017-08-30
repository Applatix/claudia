import { Injectable } from '@angular/core';
import { Http, URLSearchParams } from '@angular/http';
import { Bucket, Report } from '../model';
import { Config } from '../model/config.model';

@Injectable()
export class ConfigService {

    constructor(private http: Http) {
        // do something
    }

    public async getReportConfig(): Promise<Report> {
        return await this.http.get(`/reports`).map((res) => res.json().data[0]).toPromise();
    }


    public async createReportConfig(data: Report): Promise<Report[]> {
        return await this.http.post('/reports', data).map((res) => {
            return res.json();
        }).toPromise();
    }

    public async updateReportConfig(id: string, data: Report): Promise<any[]> {
        return await this.http.put(`/reports/${id}`, data).map((res) => {
            return res.json();
        }).toPromise();
    }

    public async deleteBucketFromReport(reportId: string, id: string): Promise<any> {
        return await this.http.delete(`/reports/${reportId}/buckets/${id}`).map((res) => {
            return res.json();
        }).toPromise();
    }

    public async createBucket(reportId: string, data: Bucket): Promise<Bucket> {
        return await this.http.post(`/reports/${reportId}/buckets`, data).map((res) => {
            return res.json();
        }).toPromise();
    }

    public async updateBucket(reportId: string, id: string, data: Bucket): Promise<Bucket> {
        return await this.http.put(`/reports/${reportId}/buckets/${id}`, data).map((res) => {
            return res.json();
        }).toPromise();
    }

    public async getConfig(): Promise<Config> {
        return await this.http.get(`/config`).map((res) => res.json()).toPromise();
    }

    public async updateConfig(data: Config): Promise<any> {
        return await this.http.put(`/config`, data).map((res) => res.json()).toPromise();
    }
}
