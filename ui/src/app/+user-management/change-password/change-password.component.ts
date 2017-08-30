import { Component } from '@angular/core';

import { UserService } from '../../services';
import { AppError } from '../../common/shared';
import { NotificationsService } from '../../common/notifications';
import { ChangePasswordData } from './change-password-data';

@Component({
    selector: 'axc-change-password',
    templateUrl: './change-password.component.html',
})
export class ChangePasswordComponent {

    public changePasswordData: ChangePasswordData;

    constructor(private userService: UserService, private notificationService: NotificationsService) {
        this.changePasswordData = new ChangePasswordData();
    }

    public async doChangePassword(form) {
        await this.userService.changePassword(this.changePasswordData).then((res) => {
            this.notificationService.success('Password has been updated');
            this.changePasswordData = new ChangePasswordData();
            form.reset();
        }).catch((err) => {
            let appErr = <AppError> err;
            this.notificationService.error(appErr.message || 'Can not update password. Please try again later.');
        });
    }
}
