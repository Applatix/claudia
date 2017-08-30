import { NgModule } from '@angular/core';
import { RouterModule } from '@angular/router';

import { IntroLayoutComponent } from '.';
import { NotificationsModule } from '../../common/notifications';

@NgModule({
    declarations: [
        IntroLayoutComponent,
    ],
    imports: [
        RouterModule,
        NotificationsModule,
    ],
})
export class IntroLayoutModule {}
