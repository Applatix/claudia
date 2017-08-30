import { CommonModule } from '@angular/common';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { NgModule } from '@angular/core';
import { RouterModule } from '@angular/router';

import { ChangePasswordComponent } from './change-password/change-password.component';
import { UserDetailsComponent } from './user-details/user-details.component';
import { AxLibModule, AxcCommonModule } from '../common';
import { CollapseBoxModule } from '../common/collapse-box';
import { ValidateOnBlurDirective } from '../common/validate-onblur';
import { AxcPipeModule } from '../pipe/axc-pipe.module';

export const routes = [
    { path: '', component: ChangePasswordComponent, data: { title: 'Users' } },
    { path: 'change-password', component: ChangePasswordComponent, data: { title: 'Change Password' } },
    { path: 'user-details/:id', component: UserDetailsComponent, data: { title: 'User details' } },
];

@NgModule({
    declarations: [
        ChangePasswordComponent,
        UserDetailsComponent,
        ValidateOnBlurDirective,
    ],
    imports: [
        AxcPipeModule,
        AxLibModule,
        AxcCommonModule,
        CollapseBoxModule,
        CommonModule,
        FormsModule,
        ReactiveFormsModule,
        RouterModule.forChild(routes),
    ],
})
export default class UserManagementModule {

}
