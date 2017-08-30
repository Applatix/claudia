import { NgModule } from '@angular/core';
import { Http, XHRBackend, RequestOptions } from '@angular/http';
import { AxLibModule } from './ax-lib';

import { HttpInterceptor } from './http-interceptor';
import { PasswordMatcherDirective } from './password-macher';

export * from './ax-lib';

@NgModule({
    providers: [{
        provide: Http,
        useFactory: (backend: XHRBackend, options: RequestOptions) => new HttpInterceptor(backend, options),
        deps: [XHRBackend, RequestOptions],
    }, {
            provide: '',
            useValue: `${process.env.API_PROTOCOL}${process.env.API_URI}`,
        }],
    imports: [
        AxLibModule,
    ],
    declarations: [
        PasswordMatcherDirective,
    ],
    exports: [
        PasswordMatcherDirective,
    ],
})
export class AxcCommonModule {
}
