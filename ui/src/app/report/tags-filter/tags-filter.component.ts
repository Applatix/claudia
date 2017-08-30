import { Component, Input, Output, EventEmitter } from '@angular/core';

@Component({
    selector: 'axc-tags-filter',
    templateUrl: './tags-filter.component.html',
})
export class TagsFilterComponent {

    @Input()
    public tags: any[];

    @Input()
    public selectedTags: any;

    @Input()
    public typeName: string;

    @Output()
    public onRemoveTag: EventEmitter<any> = new EventEmitter();

    @Output()
    public onRefresh: EventEmitter<any> = new EventEmitter();

    close(tag) {
        this.onRemoveTag.next(tag);
        this.onRefresh.next({});
    }

    closeAll() {
        this.tags.forEach((tag: any) => {
            if (this.selectedTags.has(tag.name)) {
                this.onRemoveTag.next(tag);
            }
        });
        this.onRefresh.next({});
    }
}
