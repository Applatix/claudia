export interface GroupBy {
    display_name: string;
    key: string;
}

export interface Interval {
    display_name: string;
    key: string;
}

export const resourceMetricKey = 'Resource Count';
export const DefaultMetrics = ['Unblended Cost', 'Blended Cost', resourceMetricKey];

export const groupByDefaults: GroupBy[] = [
    {
        display_name: 'None',
        key: '',
    },
    {
        display_name: 'Account',
        key: 'accounts',
    },
    {
        display_name: 'Region',
        key: 'regions',
    },
    {
        display_name: 'Services',
        key: 'services',
    },
];

export const intervalDefaults: Interval[] = [
    {
        display_name: 'Hrs',
        key: '1h',
    },
    {
        display_name: 'Day',
        key: '1d',
    },
    {
        display_name: 'Week',
        key: '1w',
    },
    {
        display_name: 'Month',
        key: '1M',
    },
];
