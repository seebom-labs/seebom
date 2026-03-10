import { TestBed } from '@angular/core/testing';
import { RouterModule } from '@angular/router';
import { App } from './app';

describe('App', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [App, RouterModule.forRoot([])],
    }).compileComponents();
  });

  it('should create the app', () => {
    const fixture = TestBed.createComponent(App);
    const app = fixture.componentInstance;
    expect(app).toBeTruthy();
  });

  it('should render the navbar with brand', () => {
    const fixture = TestBed.createComponent(App);
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('.brand')?.textContent).toContain('SeeBOM');
  });

  it('should have navigation links', () => {
    const fixture = TestBed.createComponent(App);
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;
    const links = compiled.querySelectorAll('.nav-links a');
    expect(links.length).toBe(8);
    expect(links[0].textContent).toContain('Dashboard');
    expect(links[1].textContent).toContain('SBOMs');
    expect(links[2].textContent).toContain('Vulnerabilities');
    expect(links[3].textContent).toContain('CVE Impact');
    expect(links[4].textContent).toContain('Licenses');
    expect(links[5].textContent).toContain('Violations');
    expect(links[6].textContent).toContain('Dependencies');
    expect(links[7].textContent).toContain('VEX');
  });
});

