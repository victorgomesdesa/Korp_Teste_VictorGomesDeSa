import { HttpInterceptorFn } from '@angular/common/http';

const requestIdHeader = 'X-Request-Id';

export const requestIdInterceptor: HttpInterceptorFn = (request, next) => {
  if (request.headers.has(requestIdHeader)) {
    return next(request);
  }

  return next(
    request.clone({
      setHeaders: { [requestIdHeader]: createRequestId() }
    })
  );
};

function createRequestId(): string {
  if (typeof globalThis.crypto?.randomUUID === 'function') {
    return globalThis.crypto.randomUUID();
  }

  return `${Date.now()}-${Math.random().toString(16).slice(2)}`;
}
