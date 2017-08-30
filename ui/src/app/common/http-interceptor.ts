import { Injectable } from '@angular/core';
import { Http, XHRBackend, RequestOptions, Request, RequestOptionsArgs, Response, Headers } from '@angular/http';
import { Observable } from 'rxjs/Rx';
import { ERROR_CODES, AppError } from '../common/shared';

@Injectable()
export class HttpInterceptor extends Http {
    protected basePath = `${process.env.API_PROTOCOL}${process.env.API_URI}`;
    constructor(
        private backend: XHRBackend,
        private defaultOptions: RequestOptions) {
        super(backend, defaultOptions);
    }

    public request(url: string | Request, options?: RequestOptionsArgs): Observable<Response> {
        if (url instanceof Request) {
            url.url = this.basePath + url.url;
        } else {
            url = this.basePath + url;
        }

        options = options || {};
        options.withCredentials = true;
        let headers = new Headers();
        headers.set('Content-Type', 'application/json');
        options.headers = headers;
        return super.request(url, options).catch((res: Response) => {
            try {
                // Try to parse REST API error
                return Observable.throw(res.json());
            } catch (e) {
                // Fallback to HTTP response code if API response does not have properly formatter error
                let code = '';
                switch (res.status) {
                    case 400:
                        code = ERROR_CODES.INVALID_REQUEST;
                        break;
                    case 401:
                        code = ERROR_CODES.UNAUTHENTICATED_REQUEST;
                        break;
                    case 403:
                        code = ERROR_CODES.UNAUTHORIZED_REQUEST;
                        break;
                    case 404:
                        code = ERROR_CODES.ENTITY_NOT_FOUND;
                        break;
                    default:
                        code = ERROR_CODES.INTERNAL_ERROR;
                }
                return Observable.throw(<AppError> { code, message: res.text() });
            }
        });
    }
}
