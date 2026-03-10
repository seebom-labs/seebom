import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { provideRouter } from '@angular/router';
import { LicenseViolationsComponent } from './license-violations.component';

describe('LicenseViolationsComponent', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [LicenseViolationsComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
      ],
    }).compileComponents();
  });

  it('should create', () => {
    const fixture = TestBed.createComponent(LicenseViolationsComponent);
    const component = fixture.componentInstance;
    expect(component).toBeTruthy();
  });

  it('should have page title', () => {
    const fixture = TestBed.createComponent(LicenseViolationsComponent);
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('h1')?.textContent).toContain('License Compliance');
  });

  it('should default to violations tab', () => {
    const fixture = TestBed.createComponent(LicenseViolationsComponent);
    const component = fixture.componentInstance;
    expect(component.activeTab).toBe('violations');
  });
});

