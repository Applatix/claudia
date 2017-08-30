import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';

import { CheckboxComponent } from './checkbox';
import { CheckboxSlideComponent } from './checkbox-slide';
import { DropDownComponent, DropdownContentDirective, DropdownAnchorDirective } from './dropdown/dropdown.component';
import { DateRangeComponent } from './date-range';
import { MultiselectDropdownComponent } from './dropdown-multiselect/dropdown-multiselect.component';
import { MultiSelectSearchFilter } from './dropdown-multiselect/dropdown-multiselect.component';
import { RadioButtonComponent } from './radio-button';
import { SlidingPanelComponent } from './sliding-panel/sliding-panel.component';
import { HttpService } from './services';
import { SortArrowComponent } from './sort-arrow/sort-arrow.component';
import { HelpPanelComponent } from './help-panel/help-panel.component';
import { SwitchComponent } from './switch/switch.component';

@NgModule({
    declarations: [
        CheckboxComponent,
        CheckboxSlideComponent,
        DropDownComponent,
        MultiselectDropdownComponent,
        MultiSelectSearchFilter,
        RadioButtonComponent,
        SlidingPanelComponent,
        SortArrowComponent,
        HelpPanelComponent,
        DropdownContentDirective,
        DropdownAnchorDirective,
        DateRangeComponent,
        SwitchComponent,
    ],
    imports: [
        CommonModule,
        FormsModule,
    ],
    exports: [
        CheckboxComponent,
        CheckboxSlideComponent,
        DropDownComponent,
        MultiselectDropdownComponent,
        RadioButtonComponent,
        SlidingPanelComponent,
        SortArrowComponent,
        HelpPanelComponent,
        DropdownContentDirective,
        DropdownAnchorDirective,
        DateRangeComponent,
        SwitchComponent,
    ],
    providers: [
        HttpService,
    ],
})
export class AxLibModule {
}
