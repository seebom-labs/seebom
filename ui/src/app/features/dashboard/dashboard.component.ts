import { Component, OnInit, ChangeDetectionStrategy, ChangeDetectorRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterModule } from '@angular/router';
import { ApiService } from '../../core/api.service';
import { DashboardStats } from '../../core/api.models';
import { DonutChartComponent, DonutSegment } from '../../shared/charts/donut-chart.component';
import { HorizontalBarChartComponent, BarItem } from '../../shared/charts/horizontal-bar-chart.component';

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [CommonModule, RouterModule, DonutChartComponent, HorizontalBarChartComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="dashboard">
      <header class="dash-header">
        <h1>Dashboard</h1>
        <p class="subtitle">Software Bill of Materials — Governance Overview</p>
      </header>

      @if (stats) {
        <div class="kpi-row">
          <div class="kpi-card">
            <span class="kpi-value">{{ stats.total_sboms }}</span>
            <span class="kpi-label">SBOMs</span>
          </div>
          <div class="kpi-card">
            <span class="kpi-value">{{ stats.total_packages }}</span>
            <span class="kpi-label">Packages</span>
          </div>
          <div class="kpi-card warn">
            <span class="kpi-value">{{ stats.effective_vulnerabilities }}</span>
            <span class="kpi-label">Effective Vulns</span>
          </div>
          <div class="kpi-card ok">
            <span class="kpi-value">{{ stats.suppressed_by_vex }}</span>
            <span class="kpi-label">Suppressed by VEX</span>
          </div>
          <div class="kpi-card ok">
            <span class="kpi-value">{{ stats.exempted_packages }}</span>
            <span class="kpi-label">Exempted Licenses</span>
          </div>
          <div class="kpi-card">
            <span class="kpi-value">{{ stats.total_vex_statements }}</span>
            <span class="kpi-label">VEX Statements</span>
          </div>
        </div>

        <div class="charts-row">
          <div class="chart-card">
            <h3>Vulnerability Severity</h3>
            <app-donut-chart [segments]="severitySegments" centerLabel="Vulnerabilities" />
          </div>
          <div class="chart-card">
            <h3>License Breakdown</h3>
            <app-donut-chart [segments]="licenseSegments" centerLabel="Licenses" />
          </div>
          <div class="chart-card">
            <h3>VEX Effectiveness</h3>
            <app-donut-chart [segments]="vexSegments" centerLabel="Total" />
          </div>
        </div>

        <div class="charts-row two-col">
          <div class="chart-card">
            <h3>Vulnerabilities by Severity</h3>
            <app-horizontal-bar-chart [bars]="severityBars" />
          </div>
          <div class="chart-card">
            <h3>Packages by License Category</h3>
            <app-horizontal-bar-chart [bars]="licenseBars" />
          </div>
        </div>

        <div class="refresh-banner" *ngIf="stats.last_cve_refresh">
          <span class="refresh-label">Last CVE Refresh:</span>
          <span class="refresh-time">{{ stats.last_cve_refresh | date:'medium' }}</span>
          <span class="refresh-vulns" *ngIf="stats.new_vulns_since_refresh">
            {{ stats.new_vulns_since_refresh }} new vulns found
          </span>
          <span class="refresh-ok" *ngIf="!stats.new_vulns_since_refresh">
            ✓ No new vulnerabilities
          </span>
        </div>

        <div class="warning-banner" *ngIf="stats.archived_repos_count && stats.archived_repos_count > 0">
          <span class="warning-icon">⚠️</span>
          <span class="warning-text">
            <strong>{{ stats.archived_repos_count }}</strong> archived GitHub repositories detected in your dependencies.
            <a routerLink="/archived-packages">View Details →</a>
          </span>
        </div>

        <div class="quick-links">
          <a routerLink="/sboms" class="quick-link">Browse SBOMs <span class="arrow">→</span></a>
          <a routerLink="/vulnerabilities" class="quick-link">View Vulnerabilities <span class="arrow">→</span></a>
          <a routerLink="/license-violations" class="quick-link">License Violations <span class="arrow">→</span></a>
          <a routerLink="/cve-impact" class="quick-link">CVE Impact Search <span class="arrow">→</span></a>
        </div>
      } @else {
        <div class="loading-state">
          <div class="spinner"></div>
          <p>Loading…</p>
        </div>
      }
    </div>
  `,
  styles: [`
    .dashboard { padding: 24px; max-width: 1400px; margin: 0 auto; }
    .dash-header { margin-bottom: 24px; }
    .dash-header h1 { font-size: 1.25rem; font-weight: 700; letter-spacing: -0.02em; color: var(--text); }
    .subtitle { margin-top: 2px; color: var(--text-secondary); font-size: 0.8rem; }

    .kpi-row { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 12px; margin-bottom: 24px; }
    .kpi-card {
      display: flex; flex-direction: column; gap: 2px;
      padding: 16px 18px; background: var(--surface);
      border: 1px solid var(--border); border-radius: 4px;
    }
    .kpi-value { font-size: 1.5rem; font-weight: 700; color: var(--text); line-height: 1.2; letter-spacing: -0.02em; }
    .kpi-label { font-size: 0.7rem; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.04em; font-weight: 500; }
    .kpi-card.warn .kpi-value { color: var(--severity-critical); }
    .kpi-card.ok .kpi-value { color: var(--status-success); }

    .charts-row { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; margin-bottom: 16px; }
    .charts-row.two-col { grid-template-columns: repeat(2, 1fr); }
    .chart-card {
      background: var(--surface); border: 1px solid var(--border); border-radius: 4px; padding: 20px;
    }
    .chart-card h3 { margin: 0 0 14px; font-size: 0.8rem; font-weight: 600; color: var(--dark); text-transform: uppercase; letter-spacing: 0.03em; }

    .quick-links { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 8px; margin-top: 8px; }
    .quick-link {
      display: flex; align-items: center; justify-content: space-between;
      padding: 12px 16px; background: var(--surface); border: 1px solid var(--border);
      border-radius: 4px; text-decoration: none; color: var(--dark);
      font-size: 0.8rem; font-weight: 500; transition: border-color 0.15s;
    }
    .quick-link:hover { border-color: var(--accent); color: var(--accent); }
    .arrow { color: var(--text-muted); }
    .quick-link:hover .arrow { color: var(--accent); }

    .refresh-banner {
      display: flex; align-items: center; gap: 10px; padding: 10px 16px;
      background: var(--surface); border: 1px solid var(--border); border-radius: 4px;
      margin-bottom: 12px; font-size: 0.78rem;
    }
    .refresh-label { color: var(--text-secondary); font-weight: 500; }
    .refresh-time { color: var(--text); font-weight: 600; }
    .refresh-vulns { color: var(--severity-critical); font-weight: 600; }
    .refresh-ok { color: var(--status-success); font-weight: 600; }

    .warning-banner {
      display: flex; align-items: center; gap: 10px; padding: 12px 16px;
      background: color-mix(in srgb, var(--severity-high) 15%, transparent);
      border: 1px solid var(--severity-high); border-radius: 4px;
      margin-bottom: 12px; font-size: 0.85rem;
    }
    .warning-icon { font-size: 1.2rem; }
    .warning-text { color: var(--text); }
    .warning-text a { color: var(--accent); font-weight: 600; margin-left: 8px; }
    .warning-text a:hover { text-decoration: underline; }

    .loading-state { display: flex; flex-direction: column; align-items: center; gap: 12px; padding: 80px 0; color: var(--text-muted); }
    .spinner {
      width: 28px; height: 28px; border: 2px solid #e5e7eb; border-top-color: var(--accent);
      border-radius: 50%; animation: spin 0.7s linear infinite;
    }
    @keyframes spin { to { transform: rotate(360deg); } }
  `],
})
export class DashboardComponent implements OnInit {
  stats: DashboardStats | null = null;
  severitySegments: DonutSegment[] = [];
  licenseSegments: DonutSegment[] = [];
  vexSegments: DonutSegment[] = [];
  severityBars: BarItem[] = [];
  licenseBars: BarItem[] = [];

  constructor(private readonly api: ApiService, private readonly cdr: ChangeDetectorRef) {}

  ngOnInit(): void {
    this.api.getDashboardStats().subscribe((data) => {
      this.stats = data;
      this.buildCharts(data);
      this.cdr.markForCheck();
    });
  }

  private buildCharts(s: DashboardStats): void {
    this.severitySegments = [
      { label: 'Critical', value: s.critical_vulns, color: '#C43030' },
      { label: 'High', value: s.high_vulns, color: '#E8871E' },
      { label: 'Medium', value: s.medium_vulns, color: '#C07012' },
      { label: 'Low', value: s.low_vulns, color: '#4b5563' },
    ];
    this.severityBars = [...this.severitySegments];

    const lb = s.license_breakdown || {};
    this.licenseSegments = [
      { label: 'Permissive', value: lb['permissive'] || 0, color: '#0D6B5E' },
      { label: 'Copyleft', value: lb['copyleft'] || 0, color: '#C43030' },
      { label: 'Exempted', value: s.exempted_packages || 0, color: '#E8871E' },
      { label: 'Unknown', value: lb['unknown'] || 0, color: '#9ca3af' },
    ];
    this.licenseBars = [...this.licenseSegments];

    this.vexSegments = [
      { label: 'Effective', value: s.effective_vulnerabilities, color: '#E8871E' },
      { label: 'Suppressed', value: s.suppressed_by_vex, color: '#0D6B5E' },
    ];
  }
}

