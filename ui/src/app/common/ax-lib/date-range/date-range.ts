import * as moment from 'moment';

export class DateRange {
    private _startDate: moment.Moment;
    private _endDate: moment.Moment;
    private dateFormat: string = 'YYYY-MM-DD';

    constructor(startDate: string, endDate: string) {
        if (!startDate || !endDate) {
            throw 'Date range needs start and end dates';
        }
        this.startDate = startDate;
        this.endDate = endDate;
    }

    get startDate(): string {
        return this._startDate.format(this.dateFormat);
    }
    set startDate(val: string) {
        this._startDate = moment(val, this.dateFormat);
    }

    get endDate(): string {
        return this._endDate.format(this.dateFormat);
    }
    set endDate(val: string) {
        this._endDate = moment(val, this.dateFormat);
    }

    public setRange(startDate: Date, endDate: Date) {
        this._endDate = moment(this.getDateString(endDate), 'YYYY-MM-DD');
        this._startDate = moment(this.getDateString(startDate), 'YYYY-MM-DD');
    }

    public format(): string {
        let dateFormat = 'D MMM YYYY';
        return `${this._startDate.format(dateFormat)} - ${this._endDate.format(dateFormat)}`;
    }

    private getDateString(date: Date): string {
        return date.getFullYear() + '-' + (date.getMonth() + 1) + '-' + date.getDate();
    }
}
