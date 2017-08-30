import { Component, Input, Output, EventEmitter } from '@angular/core';

@Component({
    selector: 'axc-switch',
    templateUrl: './switch.html',
    styles: [require('./switch.scss')],
})
export class SwitchComponent {

    @Input()
    public options = [];
    @Output()
    public onOptionChanged: EventEmitter<string> = new EventEmitter<string>();
    @Input()
    public selectedValue: string;

    protected selectOption(value: string) {
        let changed = value !== this.selectedValue;
        this.selectedValue = value;
        if (changed) {
            this.onOptionChanged.emit(value);
        }
    }
}
