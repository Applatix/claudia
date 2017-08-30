import { Component } from '@angular/core';

@Component({
    selector: 'axc-settings',
    templateUrl: './settings.component.html',
    styles: [
        require('./settings.component.scss'),
    ],
})
export class SettingsComponent {
    public appVersion: string = `${process.env.VERSION}`;
    public settingType: string = 'SETUP';

    public setTab(type: string) {
        this.settingType = type;
    }
}
