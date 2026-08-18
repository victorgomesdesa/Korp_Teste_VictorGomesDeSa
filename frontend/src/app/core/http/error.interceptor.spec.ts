import { HttpClient, HttpErrorResponse, provideHttpClient, withInterceptors } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { errorInterceptor, getApiError, isConnectionError } from './error.interceptor';

describe('errorInterceptor', () => {
  let httpTesting: HttpTestingController;
  let httpClient: HttpClient;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(withInterceptors([errorInterceptor])),
        provideHttpClientTesting()
      ]
    });
    httpTesting = TestBed.inject(HttpTestingController);
    httpClient = TestBed.inject(HttpClient);
  });

  afterEach(() => httpTesting.verify());

  it('propagates the failure with the backend envelope preserved', async () => {
    const failure = firstError(httpClient.get('/test'));

    httpTesting
      .expectOne('/test')
      .flush({ code: 'INSUFFICIENT_STOCK', message: 'Estoque insuficiente.' }, {
        status: 409,
        statusText: 'Conflict'
      });

    const error = await failure;
    expect(error).toBeInstanceOf(HttpErrorResponse);
    expect(getApiError(error)).toEqual({
      code: 'INSUFFICIENT_STOCK',
      message: 'Estoque insuficiente.'
    });
  });

  it('does not swallow a successful response', async () => {
    let received: unknown;
    httpClient.get('/test').subscribe((value) => (received = value));

    httpTesting.expectOne('/test').flush({ ok: true });

    expect(received).toEqual({ ok: true });
  });

  it('reports no envelope when the body is not a domain error', async () => {
    const failure = firstError(httpClient.get('/test'));

    httpTesting
      .expectOne('/test')
      .flush('<html>gateway</html>', { status: 502, statusText: 'Bad Gateway' });

    const error = await failure;
    expect(getApiError(error)).toBeNull();
    expect(isConnectionError(error)).toBe(false);
  });

  it('identifies a request that never reached the service', async () => {
    const failure = firstError(httpClient.get('/test'));

    httpTesting.expectOne('/test').error(new ProgressEvent('error'), { status: 0 });

    const error = await failure;
    expect(isConnectionError(error)).toBe(true);
    expect(getApiError(error)).toBeNull();
  });
});

function firstError(source: { subscribe: (observer: { error: (error: unknown) => void }) => void }) {
  return new Promise<unknown>((resolve) => source.subscribe({ error: resolve }));
}
