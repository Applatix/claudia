import { Bucket } from './bucket.model';
import { Account } from './account.model';

export interface Report {
    id?: string;
    ctime?: string;
    owner_user_id?: string;
    report_name?: string;
    aws_secret_access_key?: string;
    retention_days?: number;
    buckets?: Bucket[];
    accounts?: Account[];
    status?: string;
    status_detail?: string;
}
