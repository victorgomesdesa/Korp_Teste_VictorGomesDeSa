import { HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { catchError, throwError } from 'rxjs';

import { ApiError } from '../models/api-error';

export const errorInterceptor: HttpInterceptorFn = (request, next) =>
  next(request).pipe(catchError((error: unknown) => throwError(() => error)));

export function getApiError(error: unknown): ApiError | null {
  if (!(error instanceof HttpErrorResponse) || !isApiError(error.error)) {
    return null;
  }

  return error.error;
}

// O navegador não alcançou o serviço: não há envelope de erro para traduzir.
export function isConnectionError(error: unknown): boolean {
  return error instanceof HttpErrorResponse && error.status === 0;
}

function isApiError(value: unknown): value is ApiError {
  if (typeof value !== 'object' || value === null) {
    return false;
  }

  const candidate = value as Partial<ApiError>;
  return typeof candidate.code === 'string' && typeof candidate.message === 'string';
}
