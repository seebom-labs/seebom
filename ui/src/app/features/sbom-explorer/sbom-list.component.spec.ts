import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { provideRouter } from '@angular/router';
import { SbomListComponent } from './sbom-list.component';

describe('SbomListComponent', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [SbomListComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
      ],
    }).compileComponents();
  });

  it('should create', () => {
    const fixture = TestBed.createComponent(SbomListComponent);
    const component = fixture.componentInstance;
    expect(component).toBeTruthy();
  });

  it('should have a virtual scroll viewport', () => {
    const fixture = TestBed.createComponent(SbomListComponent);
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('cdk-virtual-scroll-viewport')).toBeTruthy();
  });
});
