import { Component } from '@angular/core';

import { UserService } from '../services';
import { AppError } from '../common/shared';
import { NotificationsService } from '../common/notifications';
import { PasswordData } from './password-data';

@Component({
    selector: 'axc-password',
    templateUrl: './password.component.html',
})
export class PasswordComponent {

    public changePasswordData: PasswordData;

    constructor(private userService: UserService, private notificationService: NotificationsService) {
        this.changePasswordData = new PasswordData();
    }

    public async doChangePassword(form) {
        await this.userService.changePassword(this.changePasswordData).then((res) => {
            this.notificationService.success('Password has been updated');
            this.changePasswordData = new PasswordData();
            form.reset();
        }).catch((err) => {
            let appErr = <AppError>err;
            this.notificationService.error(appErr.message || 'Can not update password. Please try again later.');
        });
    }
}
