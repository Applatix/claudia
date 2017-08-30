import { NgModule, ApplicationRef, ErrorHandler } from '@angular/core';
import { BrowserModule } from '@angular/platform-browser';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { HttpModule, JsonpModule } from '@angular/http';
import { RouterModule } from '@angular/router';
import { removeNgStyles, createNewHosts, createInputTransfer } from '@angularclass/hmr';
import { Ng2AutoCompleteModule } from 'ng2-auto-complete';

import { ENV_PROVIDERS } from './environment';
import { ROUTES } from './app.routes';
import { AppComponent } from './app.component';
import { AppState, InternalStateType } from './app.service';
import { AuthenticationService, ServicesModule } from './services';
import { LoginComponent } from './login';
import { EulaComponent } from './eula';
import { NoContentComponent } from './no-content';
import { AxcCommonModule } from './common/index';
import { BaseLayoutModule, IntroLayoutModule } from './layout';
import { ErrorPageComponent } from './error-page/error-page.component';
import { WhoAmIResolver } from './resolvers/who-am-i.resolver';
import { HasNoSessionResolver } from './resolvers/has-no-session.resolver';
import { ReportResolver } from './resolvers/report.resolver';
import { ReportGraphModule, ReportComponent } from './report';
import { DashboardComponent } from './dashboard';
import { ConfigComponent, AWSConfigComponent, BucketComponent } from './config';
import { AxLibModule } from './common/ax-lib';
import { CollapseBoxModule } from './common/collapse-box';
import { AppErrorHandler } from './app.error-handler';
import { AxcPipeModule } from './pipe/axc-pipe.module';
import { KeysConfigComponent } from './config/keys-config/keys-config.component';
import { SettingsComponent } from './settings';
import { AccountsComponent } from './accounts';
import { PasswordComponent } from './password';

const APP_PROVIDERS = [
    AppState,
    AuthenticationService,
];


type StoreType = {
    state: InternalStateType,
    restoreInputValues: () => void,
    disposeOldHosts: () => void,
};

@NgModule({
    bootstrap: [AppComponent],
    declarations: [
        AppComponent,
        LoginComponent,
        EulaComponent,
        SettingsComponent,
        NoContentComponent,
        ErrorPageComponent,
        DashboardComponent,
        ConfigComponent,
        AWSConfigComponent,
        KeysConfigComponent,
        BucketComponent,
        ReportComponent,
        AccountsComponent,
        PasswordComponent,
    ],
    imports: [
        AxcPipeModule,
        BrowserModule,
        FormsModule,
        ReactiveFormsModule,
        HttpModule,
        JsonpModule,
        BaseLayoutModule,
        IntroLayoutModule,
        AxcCommonModule,
        ServicesModule,
        AxLibModule,
        CollapseBoxModule,
        Ng2AutoCompleteModule,
        ReportGraphModule,
        RouterModule.forRoot(ROUTES, { useHash: false }),
    ],
    providers: [
        ENV_PROVIDERS,
        APP_PROVIDERS,
        WhoAmIResolver,
        HasNoSessionResolver,
        ReportResolver,
        {
            provide: ErrorHandler,
            useClass: AppErrorHandler,
        },
    ],
})
export class AppModule {
    constructor(public appRef: ApplicationRef, public appState: AppState) { }

    public hmrOnInit(store: StoreType) {
        if (!store || !store.state) {
            return;
        }
        // set state
        this.appState.state = store.state;
        // set input values
        if ('restoreInputValues' in store) {
            let restoreInputValues = store.restoreInputValues;
            setTimeout(restoreInputValues);
        }

        this.appRef.tick();
        delete store.state;
        delete store.restoreInputValues;
    }

    public hmrOnDestroy(store: StoreType) {
        const cmpLocation = this.appRef.components.map(cmp => cmp.location.nativeElement);
        // save state
        const state = this.appState.state;
        store.state = state;
        // recreate root elements
        store.disposeOldHosts = createNewHosts(cmpLocation);
        // save input values
        store.restoreInputValues = createInputTransfer();
        // remove styles
        removeNgStyles();
    }

    public hmrAfterDestroy(store: StoreType) {
        store.disposeOldHosts();
        delete store.disposeOldHosts;
    }
}
