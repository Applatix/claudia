import {Observable, Observer} from 'rxjs';
import {Injectable, NgZone} from '@angular/core';

declare let untar;

interface Callback {(data: any): void; }

declare class EventSource {
    /* tslint:disable */
    constructor(name: string, options?: any);
    public onmessage: Callback;
    public onerror: Callback;
    public close(): void;
    /* tslint:enable */
}

enum ReadyState {
    CONNECTING = 0,
    OPEN = 1,
    CLOSED = 2,
    DONE = 4
}

/**
 * Implements specific low level http requests e.g. reading blob or server sent events.
 */
@Injectable()
export class HttpService {
    constructor(private zone: NgZone) {}

    /**
     * Loads and unpack tarball.
     */
    public loadTar(url): Observable<{name: string, blob: Blob}> {
        return Observable.create((observer: Observer<{name: string, blob: Blob}>) => {
            let zone = this.zone;

            let xhr = new XMLHttpRequest();
            xhr.onreadystatechange = () => {
                if (xhr.readyState === ReadyState.DONE && xhr.status === 200) {
                    untar(xhr.response).progress(file => {
                        zone.run(() => observer.next(file));
                    }).then(() => {
                        zone.run(() => observer.complete());
                    }).catch(err => {
                        zone.run(() => observer.error(err));
                    });
                }
            };
            xhr.open('GET', url);
            xhr.responseType = 'arraybuffer';
            xhr.send();

            return () => { xhr.abort(); };
        });
    }

    /**
     * Reads server sent messages from specified URL.
     */
    public loadEventSource(url): Observable<string> {
        return Observable.create((observer: Observer<any>) => {
            let eventSource = new EventSource(url, {withCredentials: true});
            eventSource.onmessage = msg => observer.next(msg.data);
            eventSource.onerror = e => {
                if (e.eventPhase === ReadyState.CLOSED) {
                    observer.complete();
                } else {
                    observer.error(e);
                }
            }
            return () => {
                eventSource.close();
            };
        });
    }
}
