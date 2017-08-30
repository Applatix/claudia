export enum NotificationTypes {
    Success,
    Warning,
    Error,
}

export class Notification {
    public content: string;
    public type: NotificationTypes;
}
