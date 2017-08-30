import { NgModule } from '@angular/core';
import { AxcDatePipe } from './date.pipe';
import { DatePipe } from '@angular/common';
import { TrimTextPipe } from './trim-text.pipe';

@NgModule({
    declarations: [
        AxcDatePipe,
        TrimTextPipe,
    ],
    exports: [
        AxcDatePipe,
        TrimTextPipe,
    ],
    providers: [
        DatePipe,
    ],
})
export class AxcPipeModule {
}
