import { UsageUnit } from './usage.model';

export class Dimension {
    public display_name: string;
    public name: string;
    public dimensions: Dimension[] = [];
    public usage_units: UsageUnit[] = [];

    // This is a view specific property
    public selected?: boolean = false;

    public selectedDimensions?: Map<string, Dimension> = new Map<string, Dimension>();
    public selectedTagNames?: string[] = [];
    public _filterString?: string = '';

    set filterString(val: string) {
        this._filterString = (typeof val === 'string') ? val : '';
    }

    get filterString() {
        return this._filterString;
    }

    constructor(data) {
        if (typeof data === 'object') {
            for (let key in data) {
                if (data.hasOwnProperty(key)) {
                    if (key === 'dimensions') {
                        let arr = data[key];
                        for (let i = 0; i < arr.length; i++) {
                            this.dimensions.push(new Dimension(arr[i]));
                        }
                    } else {
                        this[key] = data[key];
                    }
                }
            }
        }

    }

    public getUsageKeys(): string[] {
        let keys = [];
        this.usage_units.forEach((value) => keys.push(value.name));
        return keys;
    }

    public addToSelectedList(dimension: Dimension) {
        if (dimension instanceof Dimension) {
            this.selectedDimensions.set(dimension.name, dimension);
            this.generateSelectedTagNames();
        }
    }

    public clearSelection() {
        this.selectedDimensions.clear();
    }

    public toggleSelection(dimension: Dimension) {
        if (dimension instanceof Dimension) {
            if (this.selectedDimensions.has(dimension.name)) {
                this.selectedDimensions.delete(dimension.name);
            } else {
                this.selectedDimensions.set(dimension.name, dimension);
            }

            this.generateSelectedTagNames();
        }
    }


    public removeDimensionFromSelection(tag: string) {
        let dimension: Dimension;

        this.selectedDimensions.forEach((value) => {
            if (value.display_name === tag) {
                dimension = value;
            }
        });

        if (this.selectedDimensions.has(dimension.name)) {
            this.selectedDimensions.delete(dimension.name);
            this.generateSelectedTagNames();
        }
    }

    public selectAll() {
        this.dimensions.forEach((value: Dimension) => {
            this.addToSelectedList(value);
        });
    }

    public getSelectedChildren() {
        let all = true;
        let values = [];
        if (!this.selected) {
            all = false;
        }
        this.dimensions.forEach((dimension, index) => {
            if (all) {
                values.push(dimension.name);
            } else {
                if (dimension.selected) {
                    values.push(dimension.name);
                }
            }
        });
        return values;
    }

    public getDimensionByName(val: string) {
        let res;
        for (let i = 0; i < this.dimensions.length; i++) {
            if (this.dimensions[i].name === val) {
                res = this.dimensions[i];
                break;
            }
        }
        return res;
    }


    public areAllSubFiltersSelected(): boolean {
        return this.selectedDimensions.size === this.dimensions.length;
    }

    private generateSelectedTagNames() {
        let arr = [];
        this.selectedDimensions.forEach((value) => {
            arr.push(value.display_name);
        });
        this.selectedTagNames = arr;
    }

}
