import { Component, OnInit, ViewEncapsulation } from '@angular/core';
import { Router, ActivatedRoute } from '@angular/router';
import { URLSearchParams } from '@angular/http';
import * as moment from 'moment';

import { DateRange } from '../common/ax-lib/date-range';
import { ReportingService } from '../services';
import { groupByDefaults, GroupBy, DefaultMetrics, intervalDefaults, Interval } from './config';
import { Dimension, Graph, Report, UsageUnit } from '../model';
import { NotificationsService } from '../common/notifications/notifications.service';

@Component({
    selector: 'axc-report',
    templateUrl: './report.component.html',
    encapsulation: ViewEncapsulation.None,
    styles: [
        require('./reporting.scss'),
    ],
})

export class ReportComponent implements OnInit {
    public reportObj: Report;

    public dateRangeFormatted: string;
    public filtersLoaded: boolean = false;

    public accountExpanded: boolean = false;
    public regionsExpanded: boolean = false;
    public servicesExpanded: boolean = false;
    public advancedExpanded: boolean = false;

    public noData: boolean = false;
    public loading: boolean = true;

    public loadedDimensions: Dimension[];
    public accountFilters: Dimension;
    public regionFilters: Dimension;
    public resourceTags: Dimension;
    public servicesFilters: Dimension;
    public showFilters: boolean = false;
    public showDrilldown: boolean = false;

    public dateRange: DateRange;
    public interval: Interval = intervalDefaults[1];
    public metric: string = DefaultMetrics[1];
    public appliedMetric: string;
    public grouping: GroupBy;
    public appliedGrouping: GroupBy;
    public selectedAccounts: Map<string, Dimension> = new Map<string, Dimension>();
    public selectedRegions: Map<string, Dimension> = new Map<string, Dimension>();
    public selectedServices: Map<string, Dimension> = new Map<string, Dimension>();
    public selectedTags: Map<string, Dimension> = new Map<string, Dimension>();

    public graphType: string = 'STACKED';
    public filterCategory: string = 'BASIC';

    public countMetricSelected: boolean = false;
    public usageMetricSelected: boolean = false;

    public resourceTagGroupBy: GroupBy[] = [];

    private defaultGroupByOptions: GroupBy[];
    private defaultIntervals: Interval[] = intervalDefaults;
    private defaultMetrics: string[] = DefaultMetrics;

    private reportData: any;
    private tableData: any[];
    private sumTotal: number;
    private firstLoad: boolean = true;
    private _tempParamStore: any;

    private reqCounter1 = 0;
    private reqCounter2 = 0;
    private reqCounter3 = 0;
    private reqCounter4 = 0;

    constructor(
        private router: Router,
        private activatedRoute: ActivatedRoute,
        private reportingService: ReportingService,
        private notificationsService: NotificationsService) {
        this.defaultGroupByOptions = groupByDefaults;
        this.grouping = this.defaultGroupByOptions[3];
    }

    public ngOnInit() {
        this.activatedRoute.data.subscribe((data: any) => {
            this.reportObj = data.report;
            if (!this.reportObj || this.reportObj.buckets.length === 0) {
                this.notificationsService.warning('Please configure the system to run reports.');
                this.router.navigateByUrl('app/config');
            } else {
                this.loadAllDefaultFilters();
            }
        });

        this.activatedRoute.params.subscribe((params: any) => {
            if (!this.firstLoad && this.filtersLoaded) {
                this.handleState(params);
            } else {
                // preserve filters for later access
                this._tempParamStore = params;
            }
        });
    }

    public toggleFilter() {
        if (this.showFilters) {
            this.runReport();
        } else {
            this.showFilters = !this.showFilters;
        }
    }

    public updateMetric(val) {
        if (this.defaultMetrics.indexOf(val) > -1) {
            this.metric = val;
        }

        let usageMetrics: string[] = this.getUsageMetricOptions();

        if (usageMetrics.indexOf(val) > -1) {
            this.metric = val;
        }
    }
    /**
     * Handle filter view interactions
     */
    public selectGrouping(value) {
        for (let i = 0; i < this.defaultGroupByOptions.length; i++) {
            if (this.defaultGroupByOptions[i].key === value) {
                this.grouping = this.defaultGroupByOptions[i];
                break;
            }
        }

        let advancedGroupByOptions = this.getAdvancedGroupByOptions();
        for (let j = 0; j < advancedGroupByOptions.length; j++) {
            if (advancedGroupByOptions[j].key === value) {
                this.grouping = advancedGroupByOptions[j];
                break;
            }
        }

    }

    public updateInterval(value) {
        for (let i = 0; i < this.defaultIntervals.length; i++) {
            if (this.defaultIntervals[i].key === value) {
                this.interval = this.defaultIntervals[i];
                break;
            }
        }
    }

    public selectAccounts(acc: string) {
        let selected: Map<string, Dimension> = new Map<string, Dimension>();
        let count = 0;
        let acl = acc.split(',');
        for (let i = 0; i < acl.length; i++) {
            let d: Dimension = this.accountFilters.getDimensionByName(acl[i]);
            if (d) {
                selected.set(d.name, d);
                count++;
            }
        }

        if (count === this.accountFilters.dimensions.length) {
            selected = new Map<string, Dimension>();
        }
        this.selectedAccounts = selected;
    }

    public selectRegions(reg) {
        let selected: Map<string, Dimension> = new Map<string, Dimension>();
        let count = 0;
        let regcl = reg.split(',');
        for (let i = 0; i < reg.length; i++) {
            let d: Dimension = this.regionFilters.getDimensionByName(regcl[i]);
            if (d) {
                selected.set(d.name, d);
                count++;
            }
        }

        if (count === this.regionFilters.dimensions.length) {
            selected = new Map<string, Dimension>();
        }
        this.selectedRegions = selected;
    }

    public selectServices(serv) {
        let selected: Map<string, Dimension> = new Map<string, Dimension>();
        let count = 0;
        let servcl = serv.split(',');
        for (let i = 0; i < servcl.length; i++) {
            let d: Dimension = this.servicesFilters.getDimensionByName(decodeURIComponent(servcl[i]));
            if (d) {
                selected.set(d.name, d);
                count++;
            }
        }

        if (count === this.servicesFilters.dimensions.length) {
            selected = new Map<string, Dimension>();
        }
        this.selectedServices = selected;
    }

    public selectResourceTags(data: Map<string, string[]>) {
        data.forEach((tagValues: string[], tagName: string) => {
            let tag: Dimension = this.resourceTags.getDimensionByName(tagName);
            if (tag) {
                tag.clearSelection();
                tagValues.forEach((tagValue: string) => {
                    let selectedTagValue = tag.getDimensionByName(tagValue);
                    if (selectedTagValue) {
                        tag.addToSelectedList(selectedTagValue);
                    }
                });
            }

        });
    }


    public getServiceForAdvancedFilter(): Dimension[] {
        let serv: Dimension;
        if (this.selectedServices.size === 1) {
            this.selectedServices.forEach((dimension) => {
                serv = dimension;
            });
        }
        return serv ? [serv] : [];
    }

    public updateStateForGroupBy(options: GroupBy[]) {

        let flag = false;
        for (let i = 0; i < this.defaultGroupByOptions.length; i++) {
            if (this.defaultGroupByOptions[i].key === this.grouping.key) {
                flag = true;
                break;
            }
        }

        if (!flag) {
            for (let j = 0; j < options.length; j++) {
                if (options[j].key === this.grouping.key) {
                    flag = true;
                    break;
                }
            }
        }

        if (!flag) {
            this.grouping = this.defaultGroupByOptions[3];
        }
    }

    public getAdvancedGroupByOptions(): GroupBy[] {
        let service: Dimension[] = this.getServiceForAdvancedFilter();
        let options: GroupBy[] = [];
        if (service.length > 0) {
            service[0].dimensions.forEach((value) => {
                options.push({ key: value.name, display_name: value.display_name });
            });
        }

        let tagFilters = this.getTagFilters();

        if (tagFilters.size > 0) {
            tagFilters.forEach((value, key) => {
                let tagType = this.resourceTags.getDimensionByName(key);
                options.push({ key: tagType.name, display_name: tagType.display_name });
            });
        }

        this.updateStateForGroupBy(options);
        return options;
    }

    public getUsageMetricOptions(): string[] {
        let service: Dimension[] = this.getServiceForAdvancedFilter();
        let metrics: string[] = [];
        if (service.length === 1) {
            metrics = service[0].getUsageKeys();
        } else {
            if (DefaultMetrics.indexOf(this.metric) === -1) {
                this.metric = DefaultMetrics[1];
            }
        }
        return metrics;
    }

    public toggleAccountSelection(account: Dimension) {
        if (this.selectedAccounts.has(account.name)) {
            this.selectedAccounts.delete(account.name);
        } else {
            this.selectedAccounts.set(account.name, account);
        }
        this.updateFilterOptions('account');
    }

    public toggleRegionSelection(region: Dimension) {
        if (this.selectedRegions.has(region.name)) {
            this.selectedRegions.delete(region.name);
        } else {
            this.selectedRegions.set(region.name, region);
        }
        this.updateFilterOptions('region');
    }

    public toggleServiceSelection(service: Dimension) {
        if (this.selectedServices.has(service.name)) {
            this.selectedServices.delete(service.name);
        } else {
            this.selectedServices.set(service.name, service);
        }
        this.updateFilterOptions('service');
    }

    public selectAllRegions() {
        this.selectedRegions = new Map<string, Dimension>();
        this.updateFilterOptions('region');
    }

    public selectAllAccounts() {
        this.selectedAccounts = new Map<string, Dimension>();
        this.updateFilterOptions('account');
    }

    public selectAllServices() {
        this.selectedServices = new Map<string, Dimension>();
        this.updateFilterOptions('service');
    }

    public updateGroupBy(val) {
        this.grouping = val;
    }

    public setInterval(val: Interval) {
        this.interval = val;
    }

    public refreshResourceTags(data: Dimension) {
        data.dimensions.forEach((tagDimension: Dimension) => {
            let curTagDimension: Dimension = this.resourceTags.getDimensionByName(tagDimension.name);
            if (curTagDimension) {
                curTagDimension.selectedDimensions.forEach((value: Dimension) => {
                    let selectedVal: Dimension = tagDimension.getDimensionByName(value.name);
                    if (selectedVal) {
                        tagDimension.addToSelectedList(selectedVal);
                    }
                });
            }
        })
        this.resourceTags = data;
    }

    /**
     * Update filters on changes and refresh information
     */
    public async updateFilterOptions(typeSelected: string) {
        let search = new URLSearchParams();
        let acc = this.getBaseDimensionFilterString(this.selectedAccounts);
        let reg = this.getBaseDimensionFilterString(this.selectedRegions);
        let serv = this.getBaseDimensionFilterString(this.selectedServices);

        switch (typeSelected) {
            case 'account':
                if (acc) {
                    search.set('accounts', acc);
                }
                this.reqCounter1++;
                let f1 = this.reqCounter1;
                let p = [
                    this.reportingService.getServiceDimensions(search),
                    this.reportingService.getRegionDimensions(search),
                    this.reportingService.getResourceTags(search),
                ];
                Promise.all(p).then((result: Dimension[]) => {
                    if (f1 === this.reqCounter1) {
                        this.servicesFilters = result[0];
                        this.regionFilters = result[1];
                        this.refreshResourceTags(result[2]);
                    }
                });
                break;

            case 'region':
                if (acc) {
                    search.set('accounts', acc);
                }

                if (reg) {
                    search.set('regions', reg);
                }
                this.reqCounter2++;
                let f2 = this.reqCounter2;
                let p2 = [
                    this.reportingService.getServiceDimensions(search),
                    this.reportingService.getResourceTags(search),
                ];

                Promise.all(p2).then((result: Dimension[]) => {
                    if (f2 === this.reqCounter2) {
                        this.servicesFilters = result[0];
                        this.refreshResourceTags(result[1]);
                    }
                });


                break;

            case 'service':
                if (acc) {
                    search.set('accounts', acc);
                }

                if (reg) {
                    search.set('regions', reg);
                }
                if (serv) {
                    search.set('services', serv);
                }
                this.reqCounter4++;
                let f3 = this.reqCounter4;
                let data = await this.reportingService.getResourceTags(search);

                if (f3 === this.reqCounter4) {
                    this.refreshResourceTags(data);
                }
                break;
            default:
                break;
        }
    }

    public formatTags(data: any) {
        return data.display_name;
    }

    public reset() {
        // Do a hard reload - safest
        window.location.href = 'app/report';
    }

    /**
     * Find a map of resourcetags selected
     */
    public getTagFilters() {
        let tagSelection: Map<string, string[]> = new Map<string, string[]>();

        this.resourceTags.dimensions.forEach((dimension: Dimension) => {
            let arr = [];
            if (dimension.selectedDimensions.size > 0) {
                dimension.selectedDimensions.forEach((dim: Dimension) => {
                    arr.push(dim.name);
                });
                tagSelection.set(dimension.name, arr);
            }
        });
        return tagSelection;
    }
    /**
     * Trigger Report
     */
    public runReport(drill?: any[]) {
        // Close the sliding panel
        this.showFilters = false;
        this.showDrilldown = false;

        let search = new URLSearchParams();
        let acc = this.getBaseDimensionFilterString(this.selectedAccounts);
        let reg = this.getBaseDimensionFilterString(this.selectedRegions);
        let serv = this.getBaseDimensionFilterString(this.selectedServices);
        let selectedTags = this.getTagFilters();

        if (this.selectedServices.size === 1) {
            let servDim = this.selectedServices.get(serv);
            let map = new Map<string, string>();
            servDim.dimensions.forEach((filter) => {
                let selectedChildren = Array.from(filter.selectedDimensions.keys());
                if (selectedChildren.length > 0) {
                    map.set(filter.name, selectedChildren.join(','));
                }
            });
            map.forEach((val, key) => {
                search.set(key, val);
            });
        }

        if (acc) {
            search.set('accounts', acc);
        }
        if (reg) {
            search.set('regions', reg);
        }
        if (serv) {
            search.set('services', serv);
        }

        if (selectedTags.size > 0) {
            selectedTags.forEach((selection, key) => {
                search.set(key, selection.join(','));
            });
        }
        search.set('from', this.dateRange.startDate);
        search.set('to', this.dateRange.endDate);
        search.set('interval', this.interval.key);
        search.set('group_by', this.grouping.key);

        if (this.metric === DefaultMetrics[1]) {
            search.set('blended', 'true');
        }

        if (drill && drill.length > 0) {
            search.set(drill[0], drill[1]);
        }

        this.router.navigate(['app/report', {
            query: search.toString(),
            metric: this.metric,
        }]);
    }


    public async showReport(search: URLSearchParams, metric) {
        let countAPI = this.countMetricSelected;
        let usageAPI = this.usageMetricSelected;

        this.tableData = null;
        this.sumTotal = null;
        this.loading = true;
        this.reqCounter3++;
        let f3 = this.reqCounter3;

        // for promise.all
        let p = [];

        if (!usageAPI) {
            p = [this.reportingService.runReport(search, countAPI)];
            if (countAPI) {
                p.push(this.reportingService.runReportWithoutInterval(search, countAPI));
            }
        } else {
            search.paramsMap.delete('metric');
            let serviceName = decodeURIComponent(search.paramsMap.get('services')[0]);
            search.paramsMap.delete('services');
            p.push(this.reportingService.runUsageReport(this.metric, serviceName, search));
        }

        Promise.all(p).then((result) => {
            let data: Graph[] = result[0];
            let tableData: Graph[] = countAPI ? result[1] : result[0];
            if (f3 === this.reqCounter3) {
                this.loading = false;
                if (data.length > 0) {
                    this.noData = false;
                    this.showReportView(data, tableData);
                } else {
                    this.noData = true;
                }
            }
        }, (error) => {
            this.loading = false;
            this.notificationsService.error(error.message);
            this.noData = true;
        });
    }

    public getBaseDimensionFilterString(dimensions: Map<string, Dimension>) {
        let arr = [];
        dimensions.forEach((d, k) => {
            arr.push(k);
        });
        return arr.join(',');
    }

    public doDrillDown(data: any[]) {
        this.runReport([data[2].dimension, data[2].name])
    }

    public setGraphType(type: string) {
        this.graphType = type;
    }

    private showReportView(graphData: Graph[], data: any) {
        let isCountData = this.metric === this.defaultMetrics[2];
        let payload = { graphData, isCountData, resourceSummary: null };
        if (isCountData) {
            payload.resourceSummary = data;
        }
        this.reportData = payload;

        let d = this.reportingService.generateSummary(data, isCountData);
        this.tableData = d.graph;
        this.sumTotal = isCountData ? d.total : (d.total).toFixed(2);
    }

    private resetFilters() {
        this.servicesFilters = this.loadedDimensions[0];
        this.accountFilters = this.loadedDimensions[1];
        this.regionFilters = this.loadedDimensions[2];
        this.resourceTags = this.loadedDimensions[3];
        this.selectedAccounts = new Map<string, Dimension>();
        this.selectedRegions = new Map<string, Dimension>();
        this.selectedServices = new Map<string, Dimension>();
    }

    private loadAllDefaultFilters() {
        let p = [
            this.reportingService.getServiceDimensions(),
            this.reportingService.getAccountDimensions(),
            this.reportingService.getRegionDimensions(),
            this.reportingService.getResourceTags(),
        ];

        Promise.all(p).then((result: Dimension[]) => {
            this.loadedDimensions = result;
            if (this.firstLoad) {
                this.resetFilters();
            }
            this.filtersLoaded = true;

            if (this.firstLoad && this.filtersLoaded) {
                this.handleState(this._tempParamStore);
            }
        }, (err) => {
            this.notificationsService.error('Unable to load dimensions.');
        });

    }

    private handleState(params: any) {
        let knownParams = ['accounts', 'regions', 'services', 'from', 'to', 'interval', 'group_by'];

        this.firstLoad = false;
        let q = (params && params.query) ? decodeURIComponent(params.query).split('&') : [];
        let defaultStartDate = moment().add(-1, 'month').format('YYYY-MM-DD');
        let defaultEndDate = moment().endOf('day').format('YYYY-MM-DD');

        params = params ? Object.assign({}, params) : {};
        let search: URLSearchParams = params.query ? new URLSearchParams(decodeURIComponent(params.query)) : new URLSearchParams();


        // Select accounts
        this.selectAccounts(search.paramsMap.has('accounts') ? decodeURIComponent(search.paramsMap.get('accounts')[0]) : '');

        // Select regions
        this.selectRegions(search.paramsMap.has('regions') ? decodeURIComponent(search.paramsMap.get('regions')[0]) : '');


        // Select services
        this.selectServices(search.paramsMap.has('services') ? decodeURIComponent(search.paramsMap.get('services')[0]) : '');

        let resourceTagParams = new Map<string, string[]>();

        search.paramsMap.forEach((value, key) => {
            if (knownParams.indexOf(key) === -1) {
                resourceTagParams.set(key, value[0].split(','));
            }
        });

        this.selectResourceTags(resourceTagParams);

        // Select Advanced Filters - if only one service was selected
        if (this.selectedServices.size === 1) {
            this.getServiceForAdvancedFilter()[0].dimensions.forEach((dimension) => {
                if (search.paramsMap.has(dimension.name)) {
                    if (search.paramsMap.get(dimension.name).length > 0) {
                        let options = search.paramsMap.get(dimension.name)[0].split(',');
                        if (options.length > 0) {
                            dimension.clearSelection();
                        }
                        for (let i = 0; i < options.length; i++) {
                            let selectedOption = dimension.getDimensionByName(options[i]);
                            if (selectedOption) {
                                dimension.addToSelectedList(selectedOption);
                            }
                        }
                    }
                }
            });
        }

        if (!search.get('from')) {
            q.push('from=' + defaultStartDate);
        }

        if (!search.get('to')) {
            q.push('to=' + defaultEndDate);
        }

        if (search.get('interval')) {
            this.updateInterval(search.get('interval'));
        } else {
            q.push('interval=' + this.interval.key);
        }

        if (search.get('group_by')) {
            this.selectGrouping(search.get('group_by'));
        } else {
            q.push('group_by=' + this.grouping.key);
        }
        this.appliedGrouping = this.grouping;

        if (Array.isArray(q) && q.length > 0) {
            search = new URLSearchParams(q.join('&'));
        }

        if (params.metric) {
            this.updateMetric(params.metric);
        }

        if (this.metric === DefaultMetrics[1]) {
            search.set('blended', 'true');
        }

        if (this.metric === DefaultMetrics[2]) {
            this.countMetricSelected = true;
        } else {
            this.countMetricSelected = false;
        }

        let usageMetrics: string[] = this.getUsageMetricOptions();
        if (usageMetrics.indexOf(this.metric) > -1) {
            this.usageMetricSelected = true;
        } else {
            this.usageMetricSelected = false;
        }
        this.appliedMetric = this.metric;


        this.dateRange = new DateRange(search.get('from'), search.get('to'));
        this.dateRangeFormatted = this.dateRange.format();


        this.showReport(search, this.metric);
    }

}
