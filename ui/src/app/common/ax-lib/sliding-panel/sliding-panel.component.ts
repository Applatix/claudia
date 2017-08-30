import { Component, EventEmitter, Input, Output } from '@angular/core';

@Component({
    selector: 'ax-sliding-panel',
    templateUrl: './sliding-panel.component.html',
    styles: [require('./_sliding-panel.component.scss')],
})
export class SlidingPanelComponent {
    @Input()
    public show: boolean = false;
    @Input()
    public position: 'left' | 'right' = 'left';
    @Input()
    public hasCloseButton: boolean = true;
    @Output()
    public closeButtonClick: EventEmitter<SlidingPanelComponent> = new EventEmitter<SlidingPanelComponent>();

    public onCloseButtonClick() {
        this.closeButtonClick.emit(this);
    }
}
