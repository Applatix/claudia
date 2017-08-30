import { Component, Input } from '@angular/core';
import { Router, ActivatedRoute } from '@angular/router';

import * as layout from '../../../layout';
import { AuthenticationService, UserService } from '../../../services';

@Component({
    selector: 'axc-topbar',
    templateUrl: './topbar.component.html',
    styles: [
        require('./topbar.component.scss'),
    ],
})
export class TopbarComponent {

    @Input()
    public settings: layout.LayoutSettings;

    @Input()
    public title: string;

    constructor(
        private router: Router,
        private activatedRoute: ActivatedRoute,
        private authenticationService: AuthenticationService,
        private userService: UserService) {
    }

    public async doLogout() {
        await this.authenticationService.doLogout();
    }
    public goToReport() {
        window.location.href = 'app/report';
    }
}
