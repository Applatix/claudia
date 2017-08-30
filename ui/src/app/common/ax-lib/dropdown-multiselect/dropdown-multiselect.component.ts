import { Component, Pipe, OnInit, DoCheck, HostListener, Input, ElementRef, Output, EventEmitter, forwardRef, IterableDiffers } from '@angular/core';
import { NG_VALUE_ACCESSOR, ControlValueAccessor } from '@angular/forms';

const MULTISELECT_VALUE_ACCESSOR: any = {
    provide: NG_VALUE_ACCESSOR,
    useExisting: forwardRef(() => MultiselectDropdownComponent), // tslint:disable-line
    multi: true,
};

export interface MultiSelectOption {
    id: number;
    name: string;
    selected: boolean;
}

export interface MultiSelectSettings {
    pullRight?: boolean;
    enableSearch?: boolean;
    checkedStyle?: 'checkboxes' | 'fa';
    buttonClasses?: string;
    selectionLimit?: number;
    closeOnSelect?: boolean;
    autoUnselect?: boolean;
    showCheckAll?: boolean;
    showUncheckAll?: boolean;
    dynamicTitleMaxItems?: number;
    maxHeight?: string;
}

export interface MultiSelectTexts {
    checkAll?: string;
    uncheckAll?: string;
    checked?: string;
    checkedPlural?: string;
    searchPlaceholder?: string;
    defaultTitle?: string;
}

@Pipe({
    name: 'axcSearchFilter',
})
export class MultiSelectSearchFilter {
    public transform(options: MultiSelectOption[], args: string): MultiSelectOption[] {
        return options.filter((option: MultiSelectOption) => option.name.toLowerCase().indexOf((args || '').toLowerCase()) > -1);
    }
}

@Component({
    selector: 'axc-multiselect-dropdown',
    providers: [MULTISELECT_VALUE_ACCESSOR],
    styles: [
        require('./dropdown-multiselect.scss'),
    ],
    templateUrl: './dropdown-multiselect.html',
})
export class MultiselectDropdownComponent implements OnInit, DoCheck, ControlValueAccessor {

    @Input() public options: MultiSelectOption[];
    @Input() public settings: MultiSelectSettings;
    @Input() public texts: MultiSelectTexts;
    @Output() public selectionLimitReached = new EventEmitter();

    public model: number[];
    public title: string;
    public differ: any;
    public numSelected: number = 0;
    public isVisible: boolean = false;
    public searchFilterText: string = '';
    public defaultSettings: MultiSelectSettings = {
        pullRight: false,
        enableSearch: false,
        checkedStyle: 'checkboxes',
        buttonClasses: 'ax-btn',
        selectionLimit: 0,
        closeOnSelect: false,
        autoUnselect: false,
        showCheckAll: false,
        showUncheckAll: false,
        dynamicTitleMaxItems: 3,
        maxHeight: '300px',
    };
    public defaultTexts: MultiSelectTexts = {
        checkAll: 'Check all',
        uncheckAll: 'Uncheck all',
        checked: 'checked',
        checkedPlural: 'checked',
        searchPlaceholder: 'Search...',
        defaultTitle: 'Select',
    };

    constructor(
        private element: ElementRef,
        private differs: IterableDiffers,
    ) {
        this.differ = differs.find([]).create(null);
    }

    public onModelChange: Function = (_: any) => {};
    public onModelTouched: Function = () => {};

    @HostListener('document: click', ['$event.target'])
    public onClick(target: HTMLElement) {
        let parentFound = false;
        while (target != null && !parentFound) {
            if (target === this.element.nativeElement) {
                parentFound = true;
            }
            target = target.parentElement;
        }
        if (!parentFound) {
            this.isVisible = false;
        }
    }

    public ngOnInit() {
        this.settings = Object.assign(this.defaultSettings, this.settings);
        this.texts = Object.assign(this.defaultTexts, this.texts);
        this.title = this.texts.defaultTitle;
    }

    public writeValue(value: any): void {
        if (value !== undefined) {
            this.model = value;
        }
    }

    public registerOnChange(fn: Function): void {
        this.onModelChange = fn;
    }

    public registerOnTouched(fn: Function): void {
        this.onModelTouched = fn;
    }

    public ngDoCheck() {
        let changes = this.differ.diff(this.model);
        if (changes) {
            this.updateNumSelected();
            this.updateTitle();
        }
    }

    public clearSearch() {
        this.searchFilterText = '';
    }

    public toggleDropdown() {
        this.isVisible = !this.isVisible;
    }

    public isSelected(option: MultiSelectOption): boolean {
        return this.model && this.model.indexOf(option.id) > -1;
    }

    public setSelected(event: Event, option: MultiSelectOption) {
        if (!this.model) {
            this.model = [];
        }
        let index = this.model.indexOf(option.id)
        if (index > -1) {
            this.model.splice(index, 1);
            option.selected = false;
        } else {
            option.selected = true;
            if (this.settings.selectionLimit === 0 || this.model.length < this.settings.selectionLimit) {
                this.model.push(option.id);
            } else {
                if (this.settings.autoUnselect) {
                    this.model.push(option.id);
                    this.model.shift();
                } else {
                    this.selectionLimitReached.emit(this.model.length);
                    return;
                }
            }
        }
        if (this.settings.closeOnSelect) {
            this.toggleDropdown();
        }
        this.onModelChange(this.model);
    }

    public updateNumSelected() {
        this.numSelected = this.model && this.model.length || 0;
    }

    public updateTitle() {
        if (this.numSelected === 0) {
            this.title = this.texts.defaultTitle;
        } else if (this.settings.dynamicTitleMaxItems >= this.numSelected) {
            this.title = this.options
                .filter((option: MultiSelectOption) => this.model && this.model.indexOf(option.id) > -1)
                .map((option: MultiSelectOption) => option.name)
                .join(', ');
        } else {
            this.title = this.numSelected + ' ' + (this.numSelected === 1 ? this.texts.checked : this.texts.checkedPlural);
        }
    }

    public checkAll() {
        this.model = this.options.map((option) => option.id);
        this.onModelChange(this.model);
    }

    public uncheckAll() {
        this.model = [];
        this.onModelChange(this.model);
    }
}
