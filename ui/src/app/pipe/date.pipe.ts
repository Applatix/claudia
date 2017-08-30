import { Pipe, PipeTransform } from '@angular/core';
import { DatePipe } from '@angular/common';

@Pipe({
    name: 'axcDate',
})

export class AxcDatePipe implements PipeTransform {
    private dateFormat: string = 'yyyy/MM/dd HH:mm';

    constructor (private datePipe: DatePipe) {
    }

    public transform(value: string, pattern: string) {
        return this.datePipe.transform(value, pattern ? pattern : this.dateFormat);
    }
}
