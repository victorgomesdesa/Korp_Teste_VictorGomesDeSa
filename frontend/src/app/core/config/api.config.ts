import { environment } from '../../../environments/environment';

export const apiConfig = {
  inventoryApiUrl: environment.inventoryApiUrl,
  billingApiUrl: environment.billingApiUrl,
  voiceApiUrl: environment.voiceApiUrl
} as const;
