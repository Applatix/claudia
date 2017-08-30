import { Component } from '@angular/core';
import { NotificationsService } from '.';
import { NotificationTypes, Notification } from './notification.model';

@Component({
    selector: 'axc-notifications',
    templateUrl: './notifications.component.html',
    styles: [require('./notifications.component.scss')],
})
export class NotificationsComponent {

    public notifications: Notification[];
    public notificationTypes = NotificationTypes;

    constructor(private notificationsService: NotificationsService) {
        this.notifications = notificationsService.notifications;
    }

    public close(index: number) {
        this.notificationsService.close(index);
    }
}
