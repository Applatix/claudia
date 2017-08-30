export * from './report.component';
import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { DataChartComponent } from './data-chart';
import { AdvancedFilterComponent } from './advanced-filter.component';
import { TagsFilterComponent } from './tags-filter/tags-filter.component';

@NgModule({
    declarations: [
        DataChartComponent,
        AdvancedFilterComponent,
        TagsFilterComponent,
    ],
    exports: [
        DataChartComponent,
        AdvancedFilterComponent,
        TagsFilterComponent,
    ],
    imports: [
        CommonModule,
    ],
})

export class ReportGraphModule {
}
