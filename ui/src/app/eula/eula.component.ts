import { Component, OnInit } from '@angular/core';
import { Router, ActivatedRoute } from '@angular/router';

import { Config } from '../model/config.model';
import { ConfigService, AuthenticationService } from '../services';
import { NotificationsService } from '../common/notifications/notifications.service';

@Component({
    selector: 'axc-eula',
    templateUrl: './eula.component.html',
    styles: [
        require('./eula.scss'),
    ],
})
export class EulaComponent implements OnInit {
    public config: Config;
    public fwdUrl: string = '/app/report';

    constructor(
        private router: Router,
        private activatedRoute: ActivatedRoute,
        private notificationsService: NotificationsService,
        private configService: ConfigService,
        private authenticationService: AuthenticationService) {
    }

    public async ngOnInit() {
        this.config = await this.configService.getConfig();
        this.activatedRoute.params.subscribe((params) => {
            this.fwdUrl = params['fwd'] ? params['fwd'] : this.fwdUrl;
        });
    }

    public async onAccept() {
        let c: Config = new Config();
        c.eula_accepted = true;
        await this.configService.updateConfig(c);
        this.router.navigateByUrl(this.fwdUrl);
    }

    public async onDeny() {
        await this.authenticationService.doLogout();
        this.router.navigateByUrl('base/login');
    }

    public onAlreadyAccepted() {
        this.router.navigateByUrl(this.fwdUrl);
    }
}
