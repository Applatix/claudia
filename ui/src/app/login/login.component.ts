import { Component, OnInit } from '@angular/core';
import { Router, ActivatedRoute } from '@angular/router';

import { AuthenticationService, UserService } from '../services';
import { NotificationsService } from '../common/notifications/notifications.service';
import { FIELD_PATTERNS } from '../common/shared';

const DEFAULT_LOGIN_ERROR = 'Unable to login. Please try again later.';

@Component({
    selector: 'axc-login',
    templateUrl: './login.component.html',
    styles: [
        require('./login.component.scss'),
    ],
})
export class LoginComponent implements OnInit {
    public username: string;
    public password: string;
    public newPassword: string;
    public confirmPassword: string;
    public token: string;
    public fwdUrl: string = '/app/report';

    public passwordRegex: string = FIELD_PATTERNS.PASSWORD;

    constructor(
        private router: Router,
        private activatedRoute: ActivatedRoute,
        private authenticationService: AuthenticationService,
        private notificationsService: NotificationsService,
        private userService: UserService) {
    }

    public ngOnInit() {
        // Read the url for any forward urls
        this.activatedRoute.params.subscribe((params) => {
            this.fwdUrl = params['fwd'] ? params['fwd'] : this.fwdUrl;
        });
    }

    public async doLogin() {
        try {
            let success: any = await this.authenticationService.doLogin(this.username, this.password);
            if (success.config && success.config.eula_accepted) {
                this.router.navigateByUrl(this.fwdUrl);
            } else {
                this.router.navigate(['/base/eula', {fwdUrl: this.fwdUrl}]);
            }

        } catch (err) {
            this.notificationsService.error(err.message || DEFAULT_LOGIN_ERROR);
        }
    }

    public get loginUrl(): string {
        let url = '/base/login';
        if (this.fwdUrl && this.fwdUrl !== '') {
            url += '/' + encodeURIComponent(this.fwdUrl);
        }
        return url;
    }
}
