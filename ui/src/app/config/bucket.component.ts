import { Component, OnInit, Input, Output, EventEmitter } from '@angular/core';
import { Router, ActivatedRoute } from '@angular/router';

import { ConfigService } from '../services';
import { NotificationsService } from '../common/notifications/notifications.service';
import { FIELD_PATTERNS } from '../common/shared';
import { Bucket } from '../model';

const DEFAULT_LOGIN_ERROR = 'Unable to login. Please try again later.';

@Component({
    selector: 'axc-bucket',
    templateUrl: './bucket.component.html',
    styles: [
        require('./bucket.component.scss'),
    ],
})
export class BucketComponent implements OnInit {

    public bucketId: string;
    public bucketname: string = '';
    public report_path: string = '';
    public aws_access_key_id: string = '';
    public aws_secret_access_key: string = '';

    @Output() public onSave = new EventEmitter();
    @Output() public onDelete = new EventEmitter();

    @Input()
    set bucket(val: Bucket) {

        this.updateDataFromConfig(val);
    }

    @Input()
    public identifier;

    @Input()
    public reportId: string;
    private bucketConfig: Bucket;

    constructor(
        private router: Router,
        private activatedRoute: ActivatedRoute,
        private notificationsService: NotificationsService,
        private configService: ConfigService) {
    }

    public async ngOnInit() {
        // do something
    }

    public async updateConfig() {
        let payload: Bucket = {
            bucketname: this.bucketname,
            report_path: this.report_path,
            aws_access_key_id: this.aws_access_key_id,
            aws_secret_access_key: this.aws_secret_access_key
        };

        if (this.bucketId) {
            payload.id = this.bucketId;
        }
        let data: Bucket;
        try {
            if (!this.bucketId) {
                data = await this.configService.createBucket(this.reportId, payload);
                this.notificationsService.success('Bucket added to report. Billing data will be updated soon.');
            } else {
                data = await this.configService.updateBucket(this.reportId, this.bucketConfig.id, payload);
                this.notificationsService.success('Bucket information updated. Billing data will be updated soon.');
            }
            this.updateDataFromConfig(data);
            this.onSave.emit({ bucket: this.bucketConfig, identifier: this.identifier });
        } catch (e) {
            this.notificationsService.error(e.message);
        }
    }

    public async deleteBucket() {
        try {
            await this.configService.deleteBucketFromReport(this.reportId, this.bucketConfig.id);
            this.notificationsService.success('Bucket removed successfuly');
            this.onDelete.emit({ bucket: this.bucketConfig, identifier: this.identifier });
        } catch (e) {
            this.notificationsService.error(e.message);
        }

        return false;
    }

    private updateDataFromConfig(data: Bucket) {
        this.bucketConfig = data;
        this.bucketId = this.bucketConfig.id;
        this.bucketname = this.bucketConfig.bucketname;
        this.report_path = this.bucketConfig.report_path;
        this.aws_access_key_id = this.bucketConfig.aws_access_key_id;
        this.aws_secret_access_key = this.bucketConfig.aws_secret_access_key;
    }
}
