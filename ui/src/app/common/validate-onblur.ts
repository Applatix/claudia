import { Directive, HostListener } from '@angular/core';
import { NgControl } from '@angular/forms';

@Directive({
    selector: '[axc-validate-onblur]',
})
export class ValidateOnBlurDirective {
    constructor(public formControl: NgControl) {
    }

    @HostListener('focus')
    public onFocus() {
        this.formControl.control.markAsUntouched(false);
    }

    @HostListener('blur')
    public onBlur() {
        this.formControl.control.markAsTouched(true);
    }
}
