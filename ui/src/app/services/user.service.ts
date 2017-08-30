import { Injectable } from '@angular/core';
import { Jsonp, Http } from '@angular/http';

import { User } from '../model';
import { ChangePasswordData } from '../+user-management/change-password/change-password-data';

@Injectable()
export class UserService {

    constructor(private jsonp: Jsonp, private http: Http) {
    }

    public async changePassword(changePasswordData: ChangePasswordData): Promise<any> {
        return await this.http.put('/account', { password: changePasswordData.newPassword,
            current_password: changePasswordData.currentPassword }).map((res) => res.json()).toPromise();
    }

    public getUser(idForUser: number): Promise<User> {
        return new Promise((resolve) => {
            resolve(new User({
                firstName: 'Foo',
                lastName: 'Bar',
                email: 'foo@bar.com',
                status: 2,
                id: idForUser,
            }));
        });
    }

    public putUser(idForUser: number, user: User): Promise<User> {
        return new Promise((resolve) => {
            resolve(new User({
                firstName: 'Foo',
                lastName: 'Bar',
                email: 'foo@bar.com',
                status: 2,
                id: idForUser,
            }));
        });
    }

    public resetPassword(resetPassword: any): Promise<User> {
        return new Promise((resolve) => {
            resolve(new User({
                firstName: 'Foo',
                lastName: 'Bar',
                email: 'foo@bar.com',
                status: 2,
                id: 1,
            }));
        });
    }

    public confirmResetPassword(token: string, password: string): Promise<User> {
        return new Promise((resolve) => {
            resolve(new User({
                firstName: 'Foo',
                lastName: 'Bar',
                email: 'foo@bar.com',
                status: 2,
                id: 1,
            }));
        });
    }
}
