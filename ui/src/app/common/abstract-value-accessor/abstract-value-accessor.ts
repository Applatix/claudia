import { ControlValueAccessor } from '@angular/forms';

export abstract class AbstractValueAccessor implements ControlValueAccessor {

    private newValue: any = '';

    get value(): any {
        return this.newValue;
    };

    set value(v: any) {
        if (v !== this.newValue) {
            this.newValue = v;
            this.onChange(v);
        }
    }

    public writeValue(value: any) {
        this.newValue = value;
        this.onChange(value);
    }

    public onChange = (_) => {}; // tslint:disable-line

    public onTouched = () => {}; // tslint:disable-line

    public registerOnChange(fn: (_: any) => void): void {
        this.onChange = fn;
    }

    public registerOnTouched(fn: () => void): void {
        this.onTouched = fn;
    }
}
