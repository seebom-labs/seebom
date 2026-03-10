import { Component, Input, ChangeDetectionStrategy } from '@angular/core';
import { CommonModule } from '@angular/common';

export interface BarItem {
  label: string;
  value: number;
  color: string;
}

@Component({
  selector: 'app-horizontal-bar-chart',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="bar-chart">
      @for (bar of bars; track bar.label) {
        <div class="bar-row">
          <span class="bar-label">{{ bar.label }}</span>
          <div class="bar-track">
            <div class="bar-fill"
                 [style.width.%]="getWidth(bar.value)"
                 [style.background]="bar.color">
            </div>
          </div>
          <span class="bar-value">{{ bar.value }}</span>
        </div>
      }
    </div>
  `,
  styles: [`
    .bar-chart { display: flex; flex-direction: column; gap: 8px; width: 100%; }
    .bar-row { display: flex; align-items: center; gap: 10px; }
    .bar-label { min-width: 70px; font-size: 0.75rem; color: var(--text-secondary); text-align: right; }
    .bar-track { flex: 1; height: 20px; background: var(--bg); border-radius: 2px; overflow: hidden; }
    .bar-fill { height: 100%; border-radius: 2px; transition: width 0.5s ease; min-width: 2px; }
    .bar-value { min-width: 36px; font-size: 0.8rem; font-weight: 600; color: var(--text); font-variant-numeric: tabular-nums; }
  `],
})
export class HorizontalBarChartComponent {
  @Input() bars: BarItem[] = [];

  get maxValue(): number {
    return Math.max(...this.bars.map(b => b.value), 1);
  }

  getWidth(value: number): number {
    return (value / this.maxValue) * 100;
  }
}

