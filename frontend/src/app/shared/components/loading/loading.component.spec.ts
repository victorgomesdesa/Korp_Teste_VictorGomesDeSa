import { TestBed } from '@angular/core/testing';

import { LoadingComponent } from './loading.component';

describe('LoadingComponent', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [LoadingComponent] }).compileComponents();
  });

  it('announces the loading state with the given label', () => {
    const fixture = TestBed.createComponent(LoadingComponent);
    fixture.componentRef.setInput('label', 'Carregando produtos...');
    fixture.detectChanges();

    const status = fixture.nativeElement.querySelector('[role="status"]') as HTMLElement;
    expect(status.textContent).toContain('Carregando produtos...');
    expect(status.getAttribute('aria-label')).toBe('Carregando produtos...');
  });
});
