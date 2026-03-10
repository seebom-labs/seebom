import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { provideRouter } from '@angular/router';
import { VEXListComponent } from './vex-list.component';

describe('VEXListComponent', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [VEXListComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
      ],
    }).compileComponents();
  });

  it('should create', () => {
    const fixture = TestBed.createComponent(VEXListComponent);
    const component = fixture.componentInstance;
    expect(component).toBeTruthy();
  });

  it('should have page title', () => {
    const fixture = TestBed.createComponent(VEXListComponent);
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('h1')?.textContent).toContain('VEX Statements');
  });
});

