import { Component, OnInit, ViewEncapsulation } from '@angular/core';
import { ActivatedRoute } from '@angular/router';

import { Report, Account } from '../model';
import { ReportingService, ConfigService } from '../services';
import { NotificationsService } from '../common/notifications/notifications.service';

@Component({
    selector: 'axc-accounts',
    templateUrl: './accounts.component.html',
    encapsulation: ViewEncapsulation.None,
    styles: [
        require('./accounts.component.scss'),
    ],
})
export class AccountsComponent implements OnInit {

    public report: Report;
    public editedAccount: Account;

    constructor(
        private route: ActivatedRoute,
        private notificationsService: NotificationsService,
        private reportingService: ReportingService,
        private configService: ConfigService,
    ) { }

    public async ngOnInit() {
        this.report = await this.configService.getReportConfig();
    }

    public editAccount(account: Account) {
        this.editedAccount = account;
    }

    public saveAccount(index: number) {
        let newAccountData = this.editedAccount;
        this.reportingService.saveAccountName(this.report.id, this.editedAccount.aws_account_id, { name: this.editedAccount.name })
            .then(() => {
                this.report.accounts[index] = newAccountData;
                this.editedAccount = null;
                this.notificationsService.success('Account alias is updated.');
            }, () => {
                this.notificationsService.error('Something went wrong.');
            });
    }
}
