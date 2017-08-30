import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
    name: 'trimText',
})

export class TrimTextPipe implements PipeTransform {
    public transform(value: string, letters = 30) {
        let maxLength = letters;
        let ret = value;
        if (ret.length > maxLength) {
            ret = ret.substr(0, maxLength - 3) + '…';
        }
        return ret;
    }
}
