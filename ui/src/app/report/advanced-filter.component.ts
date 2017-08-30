import { Component, Input, OnInit, EventEmitter, Output } from '@angular/core';
import { Dimension } from '../model';

@Component({
    selector: 'axc-advanced-filter',
    templateUrl: './advanced-filter.component.html',
    styles: [
        require('./advanced-filter.scss'),
    ],
})
export class AdvancedFilterComponent implements OnInit {
    @Input()
    public filter: Dimension;

    @Output()
    public onChange: EventEmitter<any> = new EventEmitter();

    public selectedFilters: Map<string, Dimension> = new Map<string, Dimension>();

    public ngOnInit() {
        // do nothing
    }

}
