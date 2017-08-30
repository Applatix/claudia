import { Component, OnInit } from '@angular/core';
import { ConfigService } from '../../services/config.service';
import { Config } from '../../model/config.model';
import { NotificationsService } from '../../common/notifications/notifications.service';

@Component({
    selector: 'axc-keys-config',
    templateUrl: './keys-config.component.html',
    styles: [
        require('./keys-config.component.scss'),
    ],
})
export class KeysConfigComponent implements OnInit {

    public config: Config = new Config();

    constructor(private configService: ConfigService, private notificationsService: NotificationsService) {
    }

    public ngOnInit() {
        this.configService.getConfig().then((data: Config) => this.config = data);
    }

    public update() {
        this.configService.updateConfig(this.config).then((data: Config) => {
            this.config = data;
            this.notificationsService.success('Config was updated.');
        }, () => {
            this.notificationsService.error('Something went wrong.');
        });
    }
}
