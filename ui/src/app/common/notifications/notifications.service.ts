import { Injectable } from '@angular/core';

import { NotificationTypes, Notification } from './notification.model';

@Injectable()
export class NotificationsService {

    public notifications: Notification[] = [];

    public success(content: string) {
        this.createNotification(NotificationTypes.Success, content);
    }

    public warning(content: string) {
        this.createNotification(NotificationTypes.Warning, content);
    }

    public error(content: string) {
        this.createNotification(NotificationTypes.Error, content);
    }

    public close(index: number) {
        delete this.notifications[index];
    }

    private createNotification(type: NotificationTypes, content: string) {
        let newNotificationIndex = this.notifications.push({ content, type }) - 1;

        // Autohide
        setTimeout(() => {
            delete this.notifications[newNotificationIndex];
        }, 5000);
    }
}
