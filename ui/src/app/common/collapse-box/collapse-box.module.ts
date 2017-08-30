import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterModule } from '@angular/router';

import { CollapseBoxComponent } from './collapse-box.component';

@NgModule({
    declarations: [
        CollapseBoxComponent,
    ],
    imports: [
        RouterModule,
        CommonModule,
    ],
    exports: [
        CollapseBoxComponent,
    ],
})
export class CollapseBoxModule {
}
