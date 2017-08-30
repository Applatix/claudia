export * from './authentication.service';
export * from './user.service';
export * from './reporting.service';
export * from './config.service';

import { NgModule } from '@angular/core';
import { AuthenticationService, UserService, ReportingService, ConfigService } from '../services';

@NgModule({
    providers: [
        AuthenticationService,
        UserService,
        ReportingService,
        ConfigService,
    ],
})
export class ServicesModule {
}
