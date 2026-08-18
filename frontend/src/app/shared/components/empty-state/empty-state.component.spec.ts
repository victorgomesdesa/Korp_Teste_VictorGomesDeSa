import { TestBed } from '@angular/core/testing';

import { EmptyStateComponent } from './empty-state.component';

describe('EmptyStateComponent', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [EmptyStateComponent] }).compileComponents();
  });

  it('renders the title and the message', () => {
    const fixture = TestBed.createComponent(EmptyStateComponent);
    fixture.componentRef.setInput('title', 'Nenhum produto cadastrado.');
    fixture.componentRef.setInput('message', 'Cadastre um produto para começar.');
    fixture.detectChanges();

    const text = fixture.nativeElement.textContent;
    expect(text).toContain('Nenhum produto cadastrado.');
    expect(text).toContain('Cadastre um produto para começar.');
  });
});
