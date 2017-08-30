import { Component, ViewChild, OnInit } from '@angular/core';
import { Router, RouterOutlet, ActivatedRouteSnapshot, NavigationEnd } from '@angular/router';

export interface LayoutSettings {

}

export interface HasLayoutSettings {
    layoutSettings: LayoutSettings;
}

@Component({
    selector: 'axc-base-layout',
    templateUrl: './base-layout.component.html',
    styles: [
        require('./base-layout.component.scss'),
    ],
})
export class BaseLayoutComponent implements OnInit {

    public title: string;
    public layoutSettings: LayoutSettings;
    @ViewChild(RouterOutlet)
    public routerOutlet: RouterOutlet;

    constructor(private router: Router) { }

    public ngOnInit() {
        this.router.events.subscribe((event) => {
            if (event instanceof NavigationEnd) {
                this.title = this.getDeepestTitle(this.router.routerState.snapshot.root);
                if (this.routerOutlet.isActivated) {
                    let component: any = this.routerOutlet.component;
                    this.layoutSettings = component ? component.layoutSettings || {} : {};
                } else {
                    this.layoutSettings = null;
                }
            }
        });
    }

    private getDeepestTitle(routeSnapshot: ActivatedRouteSnapshot) {
        let title = routeSnapshot.data ? routeSnapshot.data['title'] : '';

        if (routeSnapshot.firstChild) {
            title = this.getDeepestTitle(routeSnapshot.firstChild) || title;
        }

        return title;
    }
}
