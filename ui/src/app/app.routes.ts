import { Routes } from '@angular/router';

import { LoginComponent } from './login';
import { BaseLayoutComponent, IntroLayoutComponent } from './layout';
import { ErrorPageComponent } from './error-page/error-page.component';
import { WhoAmIResolver } from './resolvers/who-am-i.resolver';
import { HasNoSessionResolver } from './resolvers/has-no-session.resolver';
import { ReportResolver } from './resolvers/report.resolver';
import { ReportComponent } from './report';
import { DashboardComponent } from './dashboard';
import { ConfigComponent } from './config';
import { AccountsComponent } from './accounts';
import { EulaComponent } from './eula';
import { SettingsComponent } from './settings';

export const ROUTES: Routes = [
    {
        path: 'app',
        component: BaseLayoutComponent,
        resolve: {
            whoAmIResolver: WhoAmIResolver,
        },
        children: [
            {
                path: 'users', loadChildren: () => System.import('./+user-management').then((comp: any) => {
                    return comp.default;
                }),
                data: { title: 'Users' },
            },
            {
                path: 'report', component: ReportComponent, resolve: {
                    report: ReportResolver,
                },
            },
            {
                path: 'accounts', component: AccountsComponent, resolve: {
                    report: ReportResolver,
                },
            },
            {
                path: 'dashboard', component: DashboardComponent,
            },
            {
                path: 'config', component: SettingsComponent,
            },
            {
                path: 'settings', component: SettingsComponent,
            },
        ],
    },
    {
        path: 'base',
        component: IntroLayoutComponent,
        children: [
            {
                path: 'login/:fwd', component: LoginComponent, resolve: {
                    hasNoSessionResolver: HasNoSessionResolver,
                },
            },
            {
                path: 'login', component: LoginComponent, resolve: {
                    hasNoSessionResolver: HasNoSessionResolver,
                },
            },
            {
                path: 'eula', component: EulaComponent,
                resolve: {
                    whoAmIResolver: WhoAmIResolver,
                },
            },
        ],
    },
    { path: 'error/:code', component: ErrorPageComponent },
    { path: 'error/:code/type/:type', component: ErrorPageComponent },
    { path: '**', redirectTo: 'base/login' },
];
