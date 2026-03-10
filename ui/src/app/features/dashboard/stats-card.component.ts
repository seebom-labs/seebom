import { Component, Input, ChangeDetectionStrategy } from '@angular/core';
import { DecimalPipe } from '@angular/common';

@Component({
  selector: 'app-stats-card',
  standalone: true,
  imports: [DecimalPipe],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="stats-card">
      <div class="icon">{{ icon }}</div>
      <div class="content">
        <span class="value">{{ value | number }}</span>
        <span class="title">{{ title }}</span>
      </div>
    </div>
  `,
  styles: [`
    .stats-card {
      display: flex;
      align-items: center;
      gap: 16px;
      padding: 20px;
      background: var(--surface);
      border-radius: 8px;
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
    }
    .icon { font-size: 2rem; }
    .content { display: flex; flex-direction: column; }
    .value { font-size: 1.5rem; font-weight: 700; color: var(--dark); }
    .title { font-size: 0.85rem; color: #666; margin-top: 4px; }
  `],
})
export class StatsCardComponent {
  @Input({ required: true }) title = '';
  @Input({ required: true }) value = 0;
  @Input() icon = '';
}

