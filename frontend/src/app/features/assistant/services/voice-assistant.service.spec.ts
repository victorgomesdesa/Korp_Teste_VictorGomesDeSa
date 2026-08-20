import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { apiConfig } from '../../../core/config/api.config';
import { VoiceAssistantService } from './voice-assistant.service';

describe('VoiceAssistantService', () => {
  const intentUrl = `${apiConfig.voiceApiUrl}/api/voice/intent`;
  let httpTesting: HttpTestingController;
  let voiceAssistantService: VoiceAssistantService;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()]
    });
    httpTesting = TestBed.inject(HttpTestingController);
    voiceAssistantService = TestBed.inject(VoiceAssistantService);
  });

  afterEach(() => httpTesting.verify());

  it('sends the recorded audio as a binary body', () => {
    const audio = new Blob(['audio'], { type: 'audio/webm' });

    voiceAssistantService.interpretAudio(audio).subscribe();

    const request = httpTesting.expectOne(intentUrl);
    expect(request.request.method).toBe('POST');
    expect(request.request.body).toBe(audio);
    expect(request.request.headers.get('Content-Type')).toBe('application/octet-stream');
    request.flush({ transcript: '', intent: { acao: 'desconhecido' } });
  });

  it('sends a typed command as JSON', () => {
    let received: unknown;
    voiceAssistantService
      .interpretText('fechar a nota número sete')
      .subscribe((result) => (received = result));

    const request = httpTesting.expectOne(intentUrl);
    expect(request.request.method).toBe('POST');
    expect(request.request.body).toEqual({ text: 'fechar a nota número sete' });

    const interpretation = {
      transcript: 'fechar a nota número sete',
      intent: { acao: 'fechar_nota', numeroNota: 7 }
    };
    request.flush(interpretation);

    expect(received).toEqual(interpretation);
  });
});
