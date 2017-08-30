import { Component, Input, Output, EventEmitter, ElementRef, OnInit } from '@angular/core';
import { DateRange } from './date-range';

@Component({
    selector: 'ax-date-range',
    templateUrl: './date-range.html',
    styles: [
        require('./_date-range.scss'),
    ],
})
export class DateRangeComponent implements OnInit {

    @Input()
    public range: DateRange;

    @Output()
    public rangeChanged: EventEmitter<DateRange> = new EventEmitter<DateRange>();

    private target;

    constructor(private el: ElementRef) {
    }

    public ngOnInit() {
        let that = this;
        this.target = $(this.el.nativeElement).find('.date-range-selector');
        this.target.dateRangePicker({
            showShortcuts: true, autoClose: true,
            shortcuts: {
                'prev-days': [3, 7, 30],
                prev: ['week', 'month', 'year'],
                'next-days': null,
                next: null,
            },
        }).bind('datepicker-change', (evt, obj) => {
            window.setTimeout(() => {
                that.updateRange(obj);
            }, 1);
        });

        this.updatePicker();
    }

    private updatePicker() {
        this.target.data('dateRangePicker').setDateRange(this.range.startDate, this.range.endDate, true);
    }

    private updateRange(obj) {
        this.range.setRange(obj.date1, obj.date2);
        this.rangeChanged.next();
    }
}
