import { Injectable, ErrorHandler, Injector } from '@angular/core';
import { Router } from '@angular/router';
import { ERROR_CODES } from './common/shared';

@Injectable()
export class AppErrorHandler extends ErrorHandler {
    private cachedRouter: Router;

    constructor(private injector: Injector) {
        super(false);
    }

    public handleError(error: any): void {
        if (error.rejection && error.rejection.code) {
            switch (error.rejection.code) {
                case ERROR_CODES.UNAUTHENTICATED_REQUEST:
                    this.process401Error();
                    break;
                case ERROR_CODES.UNAUTHORIZED_REQUEST:
                    this.process403Error();
                    break;
                case ERROR_CODES.ENTITY_NOT_FOUND:
                    this.process404Error();
                    break;
                default:
                    console.error(error);
            }
        } else {
            if (error.rejection) {
                let rejection = error.rejection;
                console.error('Unhandled Promise rejection:',
                    rejection instanceof Error ? rejection.message : rejection,
                    '; Zone:', error.zone.name,
                    '; Task:', error.task && error.task.source,
                    '; Value:', rejection, rejection instanceof Error ? rejection.stack : undefined);
            } else {
                super.handleError(error);
            }
        }
    }

    private get router(): Router {
        if (!this.cachedRouter) {
            this.cachedRouter = this.injector.get(Router);
        }
        return this.cachedRouter;
    }
    /**
     * We wrote our setTimeout call to redirect
     * We are pretty much outside of ngZone - this will bring us back into game on that.
     */
    private doRedirect(url) {
        setTimeout(() => {
            this.router.navigateByUrl(url);
        }, 1);
    }
    /**
     * Specific handler for taking care of 401 error code if needed
     */
    private process401Error() {
        if (window.location.pathname.indexOf('login') === -1) {
            let path = window.location.pathname;
            this.doRedirect('/base/login/' + encodeURIComponent(path));
        }
    }

    /**
     * Specific handler for taking care of 403 error code if needed
     */
    private process403Error() {
        this.doRedirect('/error/403/type/' + ERROR_CODES.UNAUTHORIZED_REQUEST);
    }

    /**
     * Specific handler for taking care of 404 error code if needed
     */
    private process404Error() {
        this.doRedirect('/error/404/type/' + ERROR_CODES.ENTITY_NOT_FOUND);
    }
}
