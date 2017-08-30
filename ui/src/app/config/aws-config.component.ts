import { Component, OnInit } from '@angular/core';
import { Router, ActivatedRoute } from '@angular/router';

import { ConfigService } from '../services';
import { NotificationsService } from '../common/notifications/notifications.service';
import { FIELD_PATTERNS } from '../common/shared';
import { Bucket, Report } from '../model';

const DEFAULT_LOGIN_ERROR = 'Unable to login. Please try again later.';

@Component({
    selector: 'axc-aws-config',
    templateUrl: './aws-config.component.html',
    styles: [
        require('./aws-config.component.scss'),
    ],
})
export class AWSConfigComponent implements OnInit {

    public config: Report;
    public buckets: Bucket[];
    public report_name: string = 'default';
    public retention_days: number = 365;

    constructor(
        private router: Router,
        private activatedRoute: ActivatedRoute,
        private notificationsService: NotificationsService,
        private configService: ConfigService) {
    }

    public async ngOnInit() {
        let data: Report = await this.configService.getReportConfig();
        if (data) {
            this.updateConfig(data);
        }
    }

    public updateConfig(data: Report) {
        this.config = data;
        this.report_name = this.config.report_name;
        this.retention_days = this.config.retention_days;
        this.buckets = this.config.buckets;
        if (this.buckets.length === 0) {
            this.addBucket();
        }
    }

    public async saveReportObj() {
        try {
            let report: Report;
            if (!this.config) {
                report = await this.configService.createReportConfig({
                    report_name: this.report_name,
                    retention_days: this.retention_days,
                });
            } else {
                report = await this.configService.updateReportConfig(this.config.id, {
                    report_name: this.report_name,
                    retention_days: this.retention_days,
                });
            }

            this.updateConfig(report);
        } catch (e) {
            this.notificationsService.error(e.message);
        }
    }

    public onBucketDelete(data) {
        this.buckets.splice(data.identifier, 1);
        if (this.buckets.length === 0) {
            this.addBucket();
        }
    }

    public onBucketSave(data: { bucket: Bucket, identifier: any }) {
        this.buckets[data.identifier] = data.bucket;
    }
    public addBucket() {
        this.buckets.unshift(new Bucket());
    }

}
