import { Injectable } from '@angular/core';
import { Resolve, ActivatedRouteSnapshot, RouterStateSnapshot } from '@angular/router';

import { ConfigService } from '../services/config.service';
import { User } from '../model/user.model';

/**
 * This resolves that the report configured has at least one bucket
 */
@Injectable()
export class ReportResolver implements Resolve<any> {
    constructor(private configService: ConfigService) { }

    public async resolve(
        route: ActivatedRouteSnapshot,
        state: RouterStateSnapshot,
    ): Promise<any> {
        return await this.configService.getReportConfig();
    }
}
