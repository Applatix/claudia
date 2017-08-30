import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';

import { NotificationsComponent } from './notifications.component';
import { NotificationsService } from './notifications.service';

@NgModule({
    declarations: [
        NotificationsComponent,
    ],
    imports: [
        CommonModule,
    ],
    exports: [
        NotificationsComponent,
    ],
    providers: [
        NotificationsService,
    ],
})
export class NotificationsModule {
}
