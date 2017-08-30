import { Component, forwardRef } from '@angular/core';
import { NG_VALUE_ACCESSOR } from '@angular/forms';

import { AbstractValueAccessor } from '../../abstract-value-accessor/abstract-value-accessor';

const CUSTOM_INPUT_CONTROL_VALUE_ACCESSOR = {
    provide: NG_VALUE_ACCESSOR,
    useExisting: forwardRef(() => CheckboxSlideComponent), // tslint:disable-line
    multi: true,
};

@Component({
    selector: 'axc-checkbox-slide',
    templateUrl: './checkbox-slide.component.html',
    styles: [require('./checkbox-slide.component.scss')],
    providers: [CUSTOM_INPUT_CONTROL_VALUE_ACCESSOR],
})
export class CheckboxSlideComponent extends AbstractValueAccessor {
}
