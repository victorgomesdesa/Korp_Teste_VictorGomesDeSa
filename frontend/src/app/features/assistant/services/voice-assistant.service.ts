import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { apiConfig } from '../../../core/config/api.config';
import { VoiceInterpretation } from '../models/voice-intent';

@Injectable({ providedIn: 'root' })
export class VoiceAssistantService {
  private readonly httpClient = inject(HttpClient);
  private readonly intentUrl = `${apiConfig.voiceApiUrl}/api/voice/intent`;

  // O áudio vai cru para o Worker, que transcreve com Whisper e interpreta o comando.
  interpretAudio(audio: Blob): Observable<VoiceInterpretation> {
    return this.httpClient.post<VoiceInterpretation>(this.intentUrl, audio, {
      headers: { 'Content-Type': 'application/octet-stream' }
    });
  }

  interpretText(text: string): Observable<VoiceInterpretation> {
    return this.httpClient.post<VoiceInterpretation>(this.intentUrl, { text });
  }
}
