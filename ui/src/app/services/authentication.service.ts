import { Injectable } from '@angular/core';
import { Http } from '@angular/http';
import { Router, ActivatedRouteSnapshot, RouterStateSnapshot } from '@angular/router';
import { ConfigService } from './config.service';
import { User, Config } from '../model';

@Injectable()
export class AuthenticationService {
    public currentUser: User = new User({});
    private fwdUrl: string = 'app/report';

    constructor(private router: Router, private http: Http, private configService: ConfigService) {
    }

    public getUser(): User {
        return this.currentUser || null;
    }

    public async doLogin(username: string, password: string): Promise<{ user: User, config: Config }> {
        let data = await this.http.post('/auth/login', { username, password }).map((res) => new User(res.json())).toPromise();
        this.currentUser = new User(data);
        let configData = await this.configService.getConfig();
        return { user: this.currentUser, config: configData };
    }

    public async doLogout(): Promise<any> {
        await this.http.post('/auth/logout', {}).toPromise();
        this.router.navigate(['/base/login']);
        this.currentUser = null;
        return { success: true };
    }

    public async whoAmI(): Promise<User> {
        let user = await this.http.get('/auth/identity').map((res) => new User(res.json())).toPromise();
        this.currentUser = new User(user);
        return user;
    }

    public async hasSession(
        route: ActivatedRouteSnapshot,
        state: RouterStateSnapshot): Promise<boolean> {
        let flag = false;
        try {
            await this.whoAmI();
            let configData = await this.configService.getConfig();

            if (configData.eula_accepted) {
                flag = true;
            } else {
                this.redirectToEULA();
            }
            return flag;
        } catch (e) {
            this.redirectIfSessionNotFound();
            return false;
        }
    }

    public async hasNoSession(
        route: ActivatedRouteSnapshot,
        state: RouterStateSnapshot): Promise<boolean> {
        try {
            this.currentUser = new User(await this.whoAmI());
            if (this.currentUser) {
                this.redirectIfSessionExists(decodeURIComponent(route.params['fwd'] ? route.params['fwd'] : ''));
            }
            return false;
        } catch (e) {
            return true;
        }
    }

    private redirectIfSessionExists(fwdUrl = '') {
        this.router.navigateByUrl(fwdUrl || this.fwdUrl);
    }

    private redirectIfSessionNotFound() {
        this.router.navigateByUrl('/base/login/' + encodeURIComponent(window.location.pathname));
    }

    private redirectToEULA() {
        this.router.navigate(['/base/eula', { fwdUrl: encodeURIComponent(window.location.pathname) }]);
    }
}
