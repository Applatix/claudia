import { NgModule } from '@angular/core';
import { RouterModule } from '@angular/router';

import { MsgBoxComponent } from './msg-box.component';

@NgModule({
    declarations: [
        MsgBoxComponent,
    ],
    imports: [
        RouterModule,
    ],
    exports: [
        MsgBoxComponent,
    ],
})
export class MsgBoxModule {
}
