export class User {
    public email: string;

    constructor(data?) {
        if (typeof data === 'object') {
            for (let key in data) {
                if (data.hasOwnProperty(key)) {
                    this[key] = data[key];
                }
            }
        }
    }
}
