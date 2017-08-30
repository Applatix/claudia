import { Injectable } from '@angular/core';
import { Resolve, ActivatedRouteSnapshot, RouterStateSnapshot } from '@angular/router';

import { AuthenticationService } from '../services/authentication.service';
import { User } from '../model/user.model';

/**
 * This resolves the WhoAmI information after reload
 */
@Injectable()
export class WhoAmIResolver implements Resolve<any> {
    constructor(private authenticationService: AuthenticationService) { }

    public resolve(
        route: ActivatedRouteSnapshot,
        state: RouterStateSnapshot
    ): Promise<any> {
        return this.authenticationService.hasSession(route, state);
    }
}
