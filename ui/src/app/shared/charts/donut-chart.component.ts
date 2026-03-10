import { Component, Input, ChangeDetectionStrategy } from '@angular/core';
import { CommonModule } from '@angular/common';

export interface DonutSegment {
  label: string;
  value: number;
  color: string;
}

@Component({
  selector: 'app-donut-chart',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="donut-chart">
      <svg [attr.viewBox]="'0 0 ' + size + ' ' + size" class="donut-svg">
        @for (seg of computedSegments; track seg.label) {
          <circle
            [attr.cx]="center"
            [attr.cy]="center"
            [attr.r]="radius"
            fill="none"
            [attr.stroke]="seg.color"
            [attr.stroke-width]="strokeWidth"
            [attr.stroke-dasharray]="seg.dashArray"
            [attr.stroke-dashoffset]="seg.dashOffset"
            stroke-linecap="round"
            class="donut-segment"
          />
        }
        <text [attr.x]="center" [attr.y]="center - 8" text-anchor="middle" class="center-value">
          {{ total }}
        </text>
        <text [attr.x]="center" [attr.y]="center + 12" text-anchor="middle" class="center-label">
          {{ centerLabel }}
        </text>
      </svg>
      <div class="legend">
        @for (seg of segments; track seg.label) {
          <div class="legend-item">
            <span class="legend-dot" [style.background]="seg.color"></span>
            <span class="legend-label">{{ seg.label }}</span>
            <span class="legend-value">{{ seg.value }}</span>
          </div>
        }
      </div>
    </div>
  `,
  styles: [`
    .donut-chart { display: flex; flex-direction: column; align-items: center; gap: 14px; }
    .donut-svg { width: 160px; height: 160px; transform: rotate(-90deg); }
    .donut-segment { transition: stroke-dashoffset 0.5s ease; }
    .center-value { font-size: 22px; font-weight: 700; fill: var(--text); transform: rotate(90deg); transform-origin: center; }
    .center-label { font-size: 10px; fill: var(--text-muted); transform: rotate(90deg); transform-origin: center; text-transform: uppercase; letter-spacing: 0.05em; }
    .legend { display: flex; flex-direction: column; gap: 5px; width: 100%; }
    .legend-item { display: flex; align-items: center; gap: 8px; font-size: 0.75rem; }
    .legend-dot { width: 8px; height: 8px; border-radius: 2px; flex-shrink: 0; }
    .legend-label { flex: 1; color: var(--text-secondary); }
    .legend-value { font-weight: 600; color: var(--text); font-variant-numeric: tabular-nums; }
  `],
})
export class DonutChartComponent {
  @Input() segments: DonutSegment[] = [];
  @Input() centerLabel = 'Total';
  @Input() size = 180;
  @Input() strokeWidth = 28;

  get center(): number { return this.size / 2; }
  get radius(): number { return (this.size - this.strokeWidth) / 2; }
  get circumference(): number { return 2 * Math.PI * this.radius; }
  get total(): number { return this.segments.reduce((s, seg) => s + seg.value, 0); }

  get computedSegments() {
    const total = this.total;
    if (total === 0) return [];
    let offset = 0;
    return this.segments.filter(s => s.value > 0).map(seg => {
      const pct = seg.value / total;
      const dashArray = `${pct * this.circumference} ${this.circumference}`;
      const dashOffset = -offset * this.circumference;
      offset += pct;
      return { ...seg, dashArray, dashOffset };
    });
  }
}

