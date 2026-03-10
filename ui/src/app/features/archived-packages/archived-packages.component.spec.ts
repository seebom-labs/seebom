import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { provideRouter } from '@angular/router';
import { ArchivedPackagesComponent } from './archived-packages.component';

describe('ArchivedPackagesComponent', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ArchivedPackagesComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
      ],
    }).compileComponents();
  });

  it('should create', () => {
    const fixture = TestBed.createComponent(ArchivedPackagesComponent);
    const component = fixture.componentInstance;
    expect(component).toBeTruthy();
  });

  it('should show loading initially', () => {
    const fixture = TestBed.createComponent(ArchivedPackagesComponent);
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('.loading')?.textContent).toContain('Loading');
  });

  it('should have loading state as true initially', () => {
    const fixture = TestBed.createComponent(ArchivedPackagesComponent);
    const component = fixture.componentInstance;
    expect(component.loading).toBe(true);
  });
});

