import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { LicenseOverviewComponent } from './license-overview.component';

describe('LicenseOverviewComponent', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [LicenseOverviewComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
      ],
    }).compileComponents();
  });

  it('should create', () => {
    const fixture = TestBed.createComponent(LicenseOverviewComponent);
    const component = fixture.componentInstance;
    expect(component).toBeTruthy();
  });

  it('should show category cards', () => {
    const fixture = TestBed.createComponent(LicenseOverviewComponent);
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;
    const cards = compiled.querySelectorAll('.category-card');
    expect(cards.length).toBe(4);
  });
});

