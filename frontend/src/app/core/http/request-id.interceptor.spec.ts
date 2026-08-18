import { HttpClient, provideHttpClient, withInterceptors } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { requestIdInterceptor } from './request-id.interceptor';

describe('requestIdInterceptor', () => {
  let httpTesting: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(withInterceptors([requestIdInterceptor])),
        provideHttpClientTesting()
      ]
    });
    httpTesting = TestBed.inject(HttpTestingController);
  });

  afterEach(() => httpTesting.verify());

  it('adds an X-Request-Id header', () => {
    const httpClient = TestBed.inject(HttpClient);

    httpClient.get('/test').subscribe();

    const request = httpTesting.expectOne('/test');
    expect(request.request.headers.get('X-Request-Id')).toBeTruthy();
    request.flush({});
  });
});
