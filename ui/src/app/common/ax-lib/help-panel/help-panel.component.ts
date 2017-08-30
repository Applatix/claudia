import { Component } from '@angular/core';
import { SlidingPanelComponent } from '../sliding-panel/sliding-panel.component';

@Component({
    selector: 'axc-help-panel',
    templateUrl: './help-panel.component.html',
    styles: [require('./help-panel.component.scss')],
})
export class HelpPanelComponent extends SlidingPanelComponent {
}
