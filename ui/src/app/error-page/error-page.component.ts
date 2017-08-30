import { Component, OnInit } from '@angular/core';
import { ActivatedRoute } from '@angular/router';

@Component({
    selector: 'axc-error-page',
    templateUrl: './error-page.html',
})
export class ErrorPageComponent implements OnInit {
    private msg: string = '';
    private errorCode: string = '';

    private errorType: string = 'System Error';
    constructor(
        private activatedRoute: ActivatedRoute) {
    }

    public ngOnInit() {
        this.activatedRoute.params.subscribe((params) => {
            if (params && params['code']) {
                this.errorCode = params['code'];
            }
            if (params && params['msg']) {
                this.msg = decodeURIComponent(params['msg']);
            }
            if (params && params['type']) {
                this.errorType = params['type'];
            }
        });
    }
}
