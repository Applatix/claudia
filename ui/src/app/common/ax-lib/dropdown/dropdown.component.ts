import { Component, AfterViewInit, ElementRef, Input, OnDestroy, OnInit, ViewEncapsulation, Directive } from '@angular/core';

let $ = require('jquery');
window['$'] = $;
window['jQuery'] = $;
import 'foundation-sites';

declare var Foundation: any;

let nextId = 0;

@Component({
    selector: 'ax-dropdown',
    template: `
        <div class="dropdown__anchor" [attr.data-toggle]="id">
            <ng-content select=".ax-dropdown-anchor"></ng-content>
        </div>
        <div (click)="close()" [attr.id]="id" data-dropdown class="dropdown-pane menu {{customClass}}" [ngClass]="{'dropdown-pane--menu': isMenu, 'top': isTop}">
            <ng-content select=".ax-dropdown-content"></ng-content>
        </div>
    `,
    styles: [
        require('./_dropdown.scss'),
    ],
    // Disable encapsulation to support menu styles on transcluded content.
    encapsulation: ViewEncapsulation.None,
})
export class DropDownComponent implements AfterViewInit, OnDestroy, OnInit {

    @Input()
    public autoClose: boolean = true;

    @Input()
    public isMenu: boolean = false;

    @Input()
    public customClass: string;

    private id: string;
    private dropDown: any;
    private onDocumentClickHandler;
    private isTop: boolean = false;

    constructor(private el: ElementRef) {
        this.id = `_dd_${nextId++}`;
    }

    public ngOnInit() {
        let dropdownContent = $(this.el.nativeElement).find('div[data-dropdown]');
        let content = $('.content');
        if (content && this.customClass !== 'align-date-range-dropdown'
            && dropdownContent.height() < $(this.el.nativeElement).position().top) {
            if (Math.max(content.height(), window.innerHeight) <= $(this.el.nativeElement).position().top +
                dropdownContent.height() + 250) {
                this.isTop = true;
            }
        }
    }

    public ngAfterViewInit() {
        let dropdownContent = $(this.el.nativeElement).find('div[data-dropdown]');
        this.dropDown = new Foundation.Dropdown(dropdownContent);
        this.onDocumentClickHandler = (e) => {
            let clickedEl = document.elementFromPoint(e.pageX, e.pageY);
            if (dropdownContent[0] !== clickedEl && dropdownContent.has(clickedEl).length === 0) {
                this.close();
            }
        };
        dropdownContent.on('show.zf.dropdown', () => {
            if (this.autoClose) {
                $(document).on('click', this.onDocumentClickHandler);
            }
        });
        dropdownContent.on('hide.zf.dropdown', () => {
            this.cleanHandler();
        });
    }

    public close() {
        this.dropDown.close();
    }

    public ngOnDestroy() {
        this.cleanHandler();
    }

    private cleanHandler() {
        $(document).off('click', this.onDocumentClickHandler);
    }
}


@Directive({ selector: 'ax-dropdown-anchor' })
export class DropdownAnchorDirective {}

@Directive({ selector: 'ax-dropdown-content' })
export class DropdownContentDirective {}
