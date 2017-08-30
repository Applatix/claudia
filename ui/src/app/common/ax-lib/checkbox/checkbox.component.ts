import { Component, forwardRef, Input } from '@angular/core';
import { NG_VALUE_ACCESSOR } from '@angular/forms';

import { AbstractValueAccessor } from '../../abstract-value-accessor/abstract-value-accessor';

const CUSTOM_INPUT_CONTROL_VALUE_ACCESSOR = {
    provide: NG_VALUE_ACCESSOR,
    useExisting: forwardRef(() => CheckboxComponent), // tslint:disable-line
    multi: true,
};

@Component({
    selector: 'axc-checkbox',
    templateUrl: './checkbox.component.html',
    styles: [require('./checkbox.component.scss')],
    providers: [CUSTOM_INPUT_CONTROL_VALUE_ACCESSOR],
})
export class CheckboxComponent extends AbstractValueAccessor {

    @Input()
    public disabled: boolean = false;

    @Input()
    set value(v: any) {
        super.value = v;
    }

    get value(): any {
        return super.value;
    }
}
