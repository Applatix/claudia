import { Component, Input } from '@angular/core';

@Component({
    selector: 'axc-collapse-box',
    templateUrl: './collapse-box.component.html',
    styles: [require('./collapse-box.component.scss')],
})
export class CollapseBoxComponent {
    @Input() public collapsed: boolean = false;

    public toggle() {
        if (this.collapsed) {
            this.show();
        } else {
            this.hide();
        }
    }

    private hide() {
        this.collapsed = true;
    }

    private show() {
        this.collapsed = false;
    }
}
