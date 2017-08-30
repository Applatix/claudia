import { Component, forwardRef, Input } from '@angular/core';
import { NG_VALUE_ACCESSOR } from '@angular/forms';

import { AbstractValueAccessor } from '../../abstract-value-accessor/abstract-value-accessor';

const CUSTOM_INPUT_CONTROL_VALUE_ACCESSOR = {
    provide: NG_VALUE_ACCESSOR,
    useExisting: forwardRef(() => RadioButtonComponent), // tslint:disable-line
    multi: true,
};

@Component({
    selector: 'axc-radio-button',
    templateUrl: './radio-button.component.html',
    styles: [require('./radio-button.component.scss')],
    providers: [CUSTOM_INPUT_CONTROL_VALUE_ACCESSOR],
})
export class RadioButtonComponent extends AbstractValueAccessor {

    @Input()
    public name: string;

    @Input()
    public radioValue: string;
}
