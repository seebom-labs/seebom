import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { DependencyStatsComponent } from './dependency-stats.component';

describe('DependencyStatsComponent', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [DependencyStatsComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
      ],
    }).compileComponents();
  });

  it('should create', () => {
    const fixture = TestBed.createComponent(DependencyStatsComponent);
    const component = fixture.componentInstance;
    expect(component).toBeTruthy();
  });

  it('should have page title', () => {
    const fixture = TestBed.createComponent(DependencyStatsComponent);
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('h1')?.textContent).toContain('Dependency Statistics');
  });
});

