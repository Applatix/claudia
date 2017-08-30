import { NgModule } from '@angular/core';
import { RouterModule } from '@angular/router';
import { CommonModule } from '@angular/common';

import { AxLibModule, AxcCommonModule } from '../../common';
import { BaseLayoutComponent } from '.';
import { TopbarComponent } from './topbar';
import { NotificationsModule } from '../../common/notifications';

@NgModule({
    declarations: [
        BaseLayoutComponent,
        TopbarComponent,
    ],
    imports: [
        CommonModule,
        RouterModule,
        NotificationsModule,
        AxLibModule,
        AxcCommonModule,
    ],
    exports: [
        TopbarComponent,
        AxcCommonModule,
    ],
})
export class BaseLayoutModule {}
