import { TestBed } from '@angular/core/testing';

import { ErrorMessageComponent } from './error-message.component';

describe('ErrorMessageComponent', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [ErrorMessageComponent] }).compileComponents();
  });

  it('announces the message as an alert instead of relying on colour', () => {
    const fixture = TestBed.createComponent(ErrorMessageComponent);
    fixture.componentRef.setInput('message', 'Não foi possível carregar os produtos.');
    fixture.detectChanges();

    const alert = fixture.nativeElement.querySelector('[role="alert"]') as HTMLElement;
    expect(alert.textContent).toContain('Não foi possível carregar os produtos.');
  });
});
