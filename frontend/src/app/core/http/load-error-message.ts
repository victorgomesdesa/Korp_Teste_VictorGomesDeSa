import { isConnectionError } from './error.interceptor';

// Mensagem das falhas de carregamento de página/listagem, exibidas no componente de erro.
export function loadErrorMessage(error: unknown, resource: string): string {
  if (isConnectionError(error)) {
    return 'Não foi possível conectar ao serviço. Tente novamente.';
  }

  return `Não foi possível carregar ${resource}. Tente novamente.`;
}
