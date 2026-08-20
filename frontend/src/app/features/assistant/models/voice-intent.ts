export type VoiceAction = 'criar_produto' | 'criar_nota' | 'fechar_nota' | 'desconhecido';

export interface VoiceIntentItem {
  code: string;
  quantity: number;
}

export interface VoiceIntent {
  acao: VoiceAction;
  code?: string;
  description?: string;
  balance?: number;
  price?: number;
  itens?: VoiceIntentItem[];
  numeroNota?: number;
}

export interface VoiceInterpretation {
  transcript: string;
  intent: VoiceIntent;
}
