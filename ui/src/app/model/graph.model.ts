export class Graph {
    public columns: string[] = [];
    public tags: Map<string, string> = new Map<string, string>();
    public name: string = '';
    public values: Array<any[]> = Array<any[]>();
    constructor(data) {
        if (typeof data === 'object') {
            for (let key in data) {
                if (data.hasOwnProperty(key)) {

                    this[key] = data[key];
                }
            }
        }
    }

    public getSeriesDisplayName(): string {
        let displayName = this.tags['display_name'];
        if (displayName === '') {
            return 'Misc. Charges';
        } else {
            return displayName;
        }
    }

    public getSumOfAllValues(): number {
        let sum = 0;

        for (let i = 0; i < this.values.length; i++) {
            let v = this.values[i];
            if (v.length === 2) {
                sum = sum + v[1];
            }
        }
        return sum;
    }

}
