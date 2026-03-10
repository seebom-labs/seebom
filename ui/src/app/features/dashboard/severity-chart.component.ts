import { Component, Input, ChangeDetectionStrategy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { DashboardStats } from '../../core/api.models';

@Component({
  selector: 'app-severity-chart',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="severity-chart">
      <h3>Vulnerability Severity Distribution</h3>
      <div class="bars">
        <div class="bar-row">
          <span class="label critical">Critical</span>
          <div class="bar" [style.width.%]="getPercent(stats.critical_vulns)">
            {{ stats.critical_vulns }}
          </div>
        </div>
        <div class="bar-row">
          <span class="label high">High</span>
          <div class="bar bar-high" [style.width.%]="getPercent(stats.high_vulns)">
            {{ stats.high_vulns }}
          </div>
        </div>
        <div class="bar-row">
          <span class="label medium">Medium</span>
          <div class="bar bar-medium" [style.width.%]="getPercent(stats.medium_vulns)">
            {{ stats.medium_vulns }}
          </div>
        </div>
        <div class="bar-row">
          <span class="label low">Low</span>
          <div class="bar bar-low" [style.width.%]="getPercent(stats.low_vulns)">
            {{ stats.low_vulns }}
          </div>
        </div>
      </div>
    </div>
  `,
  styles: [`
    .severity-chart {
      background: var(--surface);
      padding: 20px;
      border-radius: 8px;
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
    }
    h3 { margin: 0 0 16px; color: var(--dark); }
    .bars { display: flex; flex-direction: column; gap: 8px; }
    .bar-row { display: flex; align-items: center; gap: 12px; }
    .label { width: 70px; font-size: 0.85rem; font-weight: 600; }
    .critical { color: var(--severity-critical); }
    .high { color: var(--severity-high); }
    .medium { color: var(--severity-medium); }
    .low { color: var(--text-secondary); }
    .bar {
      background: var(--severity-critical);
      color: #fff;
      padding: 4px 8px;
      border-radius: 4px;
      font-size: 0.8rem;
      min-width: 30px;
      text-align: center;
    }
    .bar-high { background: var(--severity-high); }
    .bar-medium { background: var(--severity-medium); }
    .bar-low { background: var(--severity-low); }
  `],
})
export class SeverityChartComponent {
  @Input({ required: true }) stats!: DashboardStats;

  getPercent(value: number): number {
    const total = this.stats.total_vulnerabilities;
    if (total === 0) return 0;
    return Math.max((value / total) * 100, 5); // Min 5% width for visibility
  }
}

