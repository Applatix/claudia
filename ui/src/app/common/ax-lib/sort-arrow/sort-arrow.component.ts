import { Component, Input } from '@angular/core';

@Component({
    selector: 'axc-sort-arrow',
    templateUrl: './sort-arrow.component.html',
    styles: [require('./sort-arrow.component.scss')],
})
export class SortArrowComponent {
    @Input()
    private columnName: string;

    @Input()
    private sort: string;

    @Input()
    private sortBy: string;
}
