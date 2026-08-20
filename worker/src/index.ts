import {
  fewShotMessages,
  intentSchema,
  normalizeIntent,
  systemPrompt,
  type VoiceIntent
} from './intent';

const transcriptionModel = '@cf/openai/whisper-large-v3-turbo';
const intentModel = '@cf/meta/llama-3.3-70b-instruct-fp8-fast';
const repairModel = '@cf/deepseek-ai/deepseek-r1-distill-qwen-32b';
const maxAudioBytes = 4 * 1024 * 1024;
const maxTextCharacters = 2_000;

interface AssistantResponse {
  transcript: string;
  intent: VoiceIntent;
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const origin = corsOrigin(request, env);

    if (request.method === 'OPTIONS') {
      return new Response(null, { status: 204, headers: corsHeaders(origin) });
    }

    const url = new URL(request.url);
    if (url.pathname === '/health') {
      return json({ status: 'ok' }, 200, origin);
    }
    if (url.pathname !== '/api/voice/intent' || request.method !== 'POST') {
      return json({ code: 'NOT_FOUND', message: 'Recurso não encontrado.' }, 404, origin);
    }

    try {
      const transcript = await resolveTranscript(request, env);
      if (transcript.trim() === '') {
        return json(
          { code: 'EMPTY_TRANSCRIPT', message: 'Não foi possível entender o áudio.' },
          422,
          origin
        );
      }

      const intent = await interpret(env, transcript);
      return json({ transcript, intent } satisfies AssistantResponse, 200, origin);
    } catch (error) {
      if (error instanceof RequestError) {
        return json({ code: error.code, message: error.message }, error.status, origin);
      }
      console.error('voice intent failed', error);
      return json(
        { code: 'INTERNAL_ERROR', message: 'Não foi possível processar o comando.' },
        500,
        origin
      );
    }
  }
} satisfies ExportedHandler<Env>;

// Aceita áudio para transcrever ou texto já digitado, o que mantém a interface testável sem microfone.
async function resolveTranscript(request: Request, env: Env): Promise<string> {
  const contentType = request.headers.get('Content-Type') ?? '';

  if (contentType.includes('application/json')) {
    let body: { text?: unknown };
    try {
      body = (await request.json()) as { text?: unknown };
    } catch {
      throw new RequestError('INVALID_REQUEST', 'JSON inválido.', 400);
    }
    if (typeof body.text !== 'string') {
      throw new RequestError('INVALID_REQUEST', 'Informe um texto ou um áudio.', 400);
    }
    if (body.text.length > maxTextCharacters) {
      throw new RequestError('TEXT_TOO_LARGE', 'O comando é muito longo.', 413);
    }
    return body.text;
  }

  const audio = await request.arrayBuffer();
  if (audio.byteLength === 0) {
    throw new RequestError('INVALID_REQUEST', 'Áudio vazio.', 400);
  }
  if (audio.byteLength > maxAudioBytes) {
    throw new RequestError('AUDIO_TOO_LARGE', 'O áudio é muito longo. Grave um comando curto.', 413);
  }

  const result = await env.AI.run(transcriptionModel, {
    audio: arrayBufferToBase64(audio),
    task: 'transcribe',
    language: 'pt',
    vad_filter: true,
    initial_prompt:
      'Português do Brasil. Comando de um sistema de notas fiscais. Vocabulário: produto, código PROD, nome, estoque, valor, nota fiscal, quantidade, criar e fechar.'
  });

  return typeof result.text === 'string' ? result.text : '';
}

function arrayBufferToBase64(value: ArrayBuffer): string {
  const bytes = new Uint8Array(value);
  let binary = '';
  for (let index = 0; index < bytes.length; index += 8192) {
    binary += String.fromCharCode(...bytes.subarray(index, index + 8192));
  }
  return btoa(binary);
}

async function interpret(env: Env, transcript: string): Promise<VoiceIntent> {
  const result = await env.AI.run(intentModel, {
    messages: [
      { role: 'system', content: systemPrompt },
      ...fewShotMessages,
      { role: 'user', content: transcript }
    ],
    // Temperatura zero mantém a interpretação estável entre repetições do mesmo comando.
    temperature: 0,
    response_format: { type: 'json_schema', json_schema: intentSchema }
  });

  const intent = normalizeIntent(parseModelResponse(result));
  if (!isIncompleteIntent(intent)) {
    return intent;
  }

  return repairIncompleteIntent(env, transcript, intent);
}

function isIncompleteIntent(intent: VoiceIntent): boolean {
  if (intent.acao === 'criar_produto') {
    return (
      intent.code === undefined ||
      intent.description === undefined ||
      intent.balance === undefined ||
      intent.price === undefined
    );
  }
  if (intent.acao === 'criar_nota') {
    return intent.itens === undefined || intent.itens.length === 0;
  }
  if (intent.acao === 'fechar_nota') {
    return intent.numeroNota === undefined;
  }
  return false;
}

async function repairIncompleteIntent(
  env: Env,
  transcript: string,
  partialIntent: VoiceIntent
): Promise<VoiceIntent> {
  const result = await env.AI.run(repairModel, {
    messages: [
      {
        role: 'system',
        content: [
          systemPrompt,
          '',
          'Esta é uma revisão de uma extração incompleta. Reavalie a transcrição original em vez de copiar cegamente o JSON parcial.',
          'Use a ordem e a função dos números na frase. Em criar_produto, um número isolado entre o nome e o valor é o estoque, mesmo quando a palavra imediatamente anterior foi transcrita incorretamente.',
          'Não crie números ausentes: recupere somente números que aparecem na transcrição.'
        ].join('\n')
      },
      {
        role: 'user',
        content: `Transcrição original: ${transcript}\nExtração parcial: ${JSON.stringify(partialIntent)}`
      }
    ],
    temperature: 0,
    response_format: { type: 'json_schema', json_schema: intentSchema }
  });

  return normalizeIntent(parseModelResponse(result));
}

// O JSON Mode devolve o objeto direto em alguns modelos e uma string JSON em outros.
function parseModelResponse(result: unknown): unknown {
  if (typeof result !== 'object' || result === null) {
    return null;
  }

  const response = (result as { response?: unknown }).response;
  if (typeof response === 'string') {
    try {
      return JSON.parse(response);
    } catch {
      return null;
    }
  }

  return response ?? null;
}

class RequestError extends Error {
  constructor(
    readonly code: string,
    message: string,
    readonly status: number
  ) {
    super(message);
  }
}

function corsOrigin(request: Request, env: Env): string | null {
  return request.headers.get('Origin') === env.ALLOWED_ORIGIN ? env.ALLOWED_ORIGIN : null;
}

function corsHeaders(origin: string | null): HeadersInit {
  if (origin === null) {
    return {};
  }

  return {
    'Access-Control-Allow-Origin': origin,
    'Access-Control-Allow-Methods': 'POST, OPTIONS',
    // O frontend correlaciona todas as chamadas com X-Request-Id, inclusive as do assistente.
    'Access-Control-Allow-Headers': 'Content-Type, X-Request-Id',
    Vary: 'Origin'
  };
}

function json(body: unknown, status: number, origin: string | null): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json', ...corsHeaders(origin) }
  });
}
